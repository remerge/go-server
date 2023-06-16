# go-server

Package `server` provides a

## Install

```bash
go get github.com/remerge/go-server
```

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
