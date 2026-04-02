package version

import "time"

var Version = "1.0.0"

var BuildDate = time.Now().Format(time.RFC3339)
