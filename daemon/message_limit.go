package daemon

import "google.golang.org/grpc"

// MaxMessageBytes bounds each serialized desktop RPC message, not just its JSON field.
// Keep in sync with the desktop application's src/main/rpcLimits.ts.
const MaxMessageBytes = 10 * 1024 * 1024

func MessageSizeServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{grpc.MaxRecvMsgSize(MaxMessageBytes), grpc.MaxSendMsgSize(MaxMessageBytes)}
}
