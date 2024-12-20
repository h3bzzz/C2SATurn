package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "google.golang.org/grpc"
)

func main() {
    var (
        opts     []grpc.DialOption
        conn     *grpc.ClientConn 
        err      error
        client   grpcapi.AdminClient 
    )

    opts = append(opts, grpc.WithInsecure())
    if conn, err = grpc.Dial(fmt.Sprintf("localhost:%d", 7777), opts...); err !
        log.Fatal(err)
    }
    defer conn.Close()
    client = grpcapi.NewAdminClient(conn)

    var cmd = new(grpcapi.Command)
    cmd.In = os.Args[1] 
    ctx := context.Background() 
    cmd, err = client.RunCommand(ctx, cmd)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(cmd.Out)
}


