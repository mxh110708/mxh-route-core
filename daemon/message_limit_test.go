package daemon

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestMessageSizeRoundTrip(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(append(MessageSizeServerOptions(), grpc.UnknownServiceHandler(func(_ any, stream grpc.ServerStream) error {
		request := new(wrapperspb.BytesValue)
		if err := stream.RecvMsg(request); err != nil {
			return err
		}
		return stream.SendMsg(request)
	}))...)
	go server.Serve(listener)
	defer server.Stop()
	connection, err := grpc.NewClient("passthrough:///test", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(12*1024*1024), grpc.MaxCallSendMsgSize(12*1024*1024)))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	for _, size := range []int{5 * 1024 * 1024, MaxMessageBytes - 5, MaxMessageBytes} {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		response := new(wrapperspb.BytesValue)
		err := connection.Invoke(ctx, "/test/Echo", wrapperspb.Bytes(make([]byte, size)), response)
		cancel()
		// A bytes field this large has a five-byte protobuf envelope.
		if size == MaxMessageBytes {
			if status.Code(err) != codes.ResourceExhausted {
				t.Fatalf("oversized message: %v", err)
			}
		} else if err != nil || len(response.Value) != size {
			t.Fatalf("roundtrip %d: %v", size, err)
		}
	}
}
