package version

import "time"

var Version = "1.3.1"

var BuildDate = time.Now().Format(time.RFC3339)
