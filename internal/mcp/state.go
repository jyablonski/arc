package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"

	"github.com/jyablonski/arc/internal/filemode"
)

// State records which servers arc wrote into which provider. It is what makes
// the merge safe in both directions: a server dropped from canonical can be
// removed downstream because arc knows it put it there, and a server someone
// added by hand in Cursor is left alone because arc knows it did not.
type State struct {
	Managed map[string][]string `json:"managed"`
}

func LoadState(path string) (State, error) {
	st := State{Managed: map[string][]string{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return st, fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &st); err != nil {
		// A corrupt state file must not wedge sync. Falling back to "arc owns
		// nothing" is the conservative choice: it can leave a stale entry
		// behind, but it can never delete a server arc did not write.
		return State{Managed: map[string][]string{}}, nil
	}
	if st.Managed == nil {
		st.Managed = map[string][]string{}
	}
	return st, nil
}

func SaveState(path string, st State) error {
	if st.Managed == nil {
		st.Managed = map[string][]string{}
	}
	for k, v := range st.Managed {
		sort.Strings(v)
		st.Managed[k] = v
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, append(data, '\n'), filemode.File)
}

func (s State) Owned(provider string) []string {
	return s.Managed[provider]
}

func (s State) Owns(provider, name string) bool {
	return slices.Contains(s.Managed[provider], name)
}

func (s *State) SetOwned(provider string, names []string) {
	if s.Managed == nil {
		s.Managed = map[string][]string{}
	}
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	s.Managed[provider] = sorted
}
