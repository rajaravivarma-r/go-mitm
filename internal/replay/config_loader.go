package replay

import (
	"encoding/json"
	"os"
)

func newStructFromFile(filename string, value interface{}) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}
