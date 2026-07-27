package dxdevice

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Overridable seams for testing. Production defaults read the real host.
var (
	// sysClassDxrt is the sysfs class dir the driver creates one entry per card in.
	sysClassDxrt = "/sys/class/dxrt"

	// statusCmd runs `dxrt-cli -s` (all devices when id < 0, else `-d <id>`) and
	// returns its combined stdout+stderr. Tests override it with a fixture.
	statusCmd = func(id int) (string, error) {
		args := []string{"-s"}
		if id >= 0 {
			args = append(args, "-d", strconv.Itoa(id))
		}
		out, err := exec.Command("dxrt-cli", args...).CombinedOutput()
		return string(out), err
	}
)

var (
	reDxrtName  = regexp.MustCompile(`^dxrt(\d+)$`)
	reDevHeader = regexp.MustCompile(`^\s*\*\s*Device\s+(\d+):\s*([^,]+)`)
)

// List enumerates the NPU cards on this host. sysfs is authoritative for which
// cards exist (and thus the allocatable count); dxrt-cli metadata is merged in
// best-effort. A card present in sysfs but absent from dxrt-cli output is
// returned with Healthy=false.
func List() ([]Device, error) {
	names, err := listSysfs()
	if err != nil {
		return nil, err
	}

	// Best-effort metadata for all devices in one call. If dxrt-cli fails we
	// still return the sysfs-discovered cards (all Unhealthy).
	out, _ := statusCmd(-1)
	meta := parseStatus(out)

	devs := make([]Device, 0, len(names))
	for _, n := range names {
		id := idFromName(n)
		d := Device{ID: id, Name: n, NodePath: "/dev/" + n}
		if m, ok := meta[id]; ok {
			d.Product = m.Product
			d.RTDriver = m.RTDriver
			d.PCIeDriver = m.PCIeDriver
			d.FWVersion = m.FWVersion
			d.Board = m.Board
			d.PCIe = m.PCIe
			d.Healthy = true
		}
		devs = append(devs, d)
	}
	return devs, nil
}

// listSysfs returns sorted dxrtN entry names under sysClassDxrt.
func listSysfs() ([]string, error) {
	entries, err := os.ReadDir(sysClassDxrt)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", sysClassDxrt, err)
	}
	var names []string
	for _, e := range entries {
		if reDxrtName.MatchString(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Slice(names, func(i, j int) bool { return idFromName(names[i]) < idFromName(names[j]) })
	return names, nil
}

func idFromName(name string) int {
	m := reDxrtName.FindStringSubmatch(name)
	if m == nil {
		return -1
	}
	id, _ := strconv.Atoi(m[1])
	return id
}

// parseStatus parses `dxrt-cli -s` output into per-device metadata keyed by ID.
// Each device block starts at a `* Device N:` header; key/value lines that
// follow (until the next header) fill that device's fields.
func parseStatus(out string) map[int]Device {
	devs := map[int]Device{}
	curID := -1

	for _, ln := range strings.Split(out, "\n") {
		if m := reDevHeader.FindStringSubmatch(ln); m != nil {
			id, _ := strconv.Atoi(m[1])
			curID = id
			devs[id] = Device{
				ID:       id,
				Name:     fmt.Sprintf("dxrt%d", id),
				NodePath: fmt.Sprintf("/dev/dxrt%d", id),
				Product:  strings.TrimSpace(m[2]),
				Healthy:  true,
			}
			continue
		}
		if curID < 0 {
			continue
		}
		key, val, ok := splitKV(ln)
		if !ok {
			continue
		}
		d := devs[curID]
		switch key {
		case "RT Driver version":
			d.RTDriver = val
		case "PCIe Driver version":
			d.PCIeDriver = val
		case "FW version":
			d.FWVersion = val
		case "Board":
			d.Board = val
		case "PCIe":
			d.PCIe = val
		}
		devs[curID] = d
	}
	return devs
}

// splitKV parses a ` * <key> : <value>` line. It splits on the first ':' only,
// so values containing colons (e.g. a PCIe BDF "[85:00:00]") survive intact.
// Lines not starting with '*' or lacking a ':' are rejected.
func splitKV(ln string) (key, val string, ok bool) {
	s := strings.TrimSpace(ln)
	if !strings.HasPrefix(s, "*") {
		return "", "", false
	}
	s = strings.TrimSpace(strings.TrimPrefix(s, "*"))
	i := strings.Index(s, ":")
	if i < 0 {
		return "", "", false
	}
	return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:]), true
}
