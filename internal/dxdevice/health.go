package dxdevice

// Health reports whether a single card is currently usable. It runs
// `dxrt-cli -s -d <id>`: the card is Healthy only if the command succeeds AND
// its output contains a status block for that device. A wedged card typically
// exits non-zero or drops out of the listing, both of which read as Unhealthy.
func Health(id int) bool {
	out, err := statusCmd(id)
	if err != nil {
		return false
	}
	m := parseStatus(out)
	_, ok := m[id]
	return ok
}
