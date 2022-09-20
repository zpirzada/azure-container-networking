package zapai

import (
	"os"
	"path/filepath"
)

var metadataFile = filepath.FromSlash(os.Getenv("TEMP")) + "\\azuremetadata.json"
