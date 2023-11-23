# Go Server Framework

Package `server` provides an opinionated server framework in Go.

## Usage

```go
package main

import "github.com/remerge/go-server"

type serverHandler struct {
}

func (h *serverHandler) Handle(c *server.Connection) {
 // Handle stuff here
}

func main() {
 s, _ := server.NewServer(12345)
    s.Handler = &serverHandler{}
    s.Run()
}
```
