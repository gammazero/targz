# targz
Archive directory to tar.gz and extract tar.gz to directory

## Example

```go
package main

import (
	"os"

	"github.com/gammazero/targz"
)

func main() {
	// Create archive of backup directory.
	err := targz.Create("/tmp/myfiles/backup", "/tmp/backup.tar.gz")
	if err != nil {
		panic(err)
	}

	// Extract archive into /tmp/restore/ to create /tmp/restore/backup.
	os.MkdirAll("/tmp/restore", 0750)
	err = targz.Extract("/tmp/backup.tar.gz", "/tmp/restore")
	if err != nil {
		panic(err)
	}
}
```
