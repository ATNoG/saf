package main

import "os"

import (
    "knative.dev/serving/pkg/queue/sharedmain"
    "knative.dev/security-guard/pkg/qpoption"

	_ "ATNoG/queue-proxy-firewall/pkg/firewall"
)

func main() {
    qOpt := qpoption.NewGateQPOption()
    defer qOpt.Shutdown()

    if sharedmain.Main(qOpt.Setup) != nil {
      qOpt.Shutdown()
      os.Exit(1)
    }
}
