package host

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ensureSupportedHostOS allows running only on Ubuntu-based or Debian-based
// distributions to reduce the risk of breaking unrelated host systems.
func ensureSupportedHostOS() error {
	osRelease, err := parseOSRelease("/etc/os-release")
	if err != nil {
		return fmt.Errorf("read /etc/os-release: %w", err)
	}

	id := osRelease["ID"]
	idLike := osRelease["ID_LIKE"]
	if id == "" && idLike == "" {
		return errors.New("unsupported host OS (missing ID/ID_LIKE in /etc/os-release)")
	}

	if id == "ubuntu" || containsWord(idLike, "ubuntu") {
		return nil
	}

	if id == "debian" || containsWord(idLike, "debian") {
		if id == "debian" && !hasUbuntuArchiveKeyring() {
			return errors.New("on Debian, install ubuntu-archive-keyring first: sudo apt install ubuntu-archive-keyring")
		}
		return nil
	}

	return fmt.Errorf(
		"unsupported host OS (ID=%q, ID_LIKE=%q): kagami can only run on Ubuntu-based or Debian-based systems",
		id,
		idLike,
	)
}

func hasUbuntuArchiveKeyring() bool {
	cmd := exec.Command("dpkg-query", "-W", "-f=${Status}", "ubuntu-archive-keyring")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	status := string(out)
	return strings.Contains(status, "install ok installed")
}

func containsWord(s, want string) bool {
	for _, part := range strings.Fields(strings.ToLower(strings.TrimSpace(s))) {
		if part == want {
			return true
		}
	}
	return false
}

func parseOSRelease(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[k] = trimQuoted(v)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func trimQuoted(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "\"")
	v = strings.TrimSuffix(v, "\"")
	return strings.TrimSpace(v)
}
