package test

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/irvanmalik48/realm-api/internal/config"
	internalGRPC "github.com/irvanmalik48/realm-api/internal/grpc"
	realmv1 "github.com/irvanmalik48/realm-api/pkg/pb/realm/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

func setupGRPCTestServer(t *testing.T) (*grpc.ClientConn, func()) {
	_ = os.Setenv("STORAGE_DIR", "./test_grpc_storage")
	_ = os.Setenv("PASETO_SYMMETRIC_KEY", "707172737475767778797a7b7c7d7e7f808182838485868788898a8b8c8d8e8f")

	cfg := config.Load()
	cfg.StorageDir = "./test_grpc_storage"

	lis := bufconn.Listen(bufSize)
	server := internalGRPC.NewServer(cfg, nil)

	go func() {
		if err := server.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			t.Logf("Server exited: %v", err)
		}
	}()

	conn, err := grpc.NewClient(
		"passthrough://bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}

	cleanup := func() {
		_ = conn.Close()
		server.Stop()
		_ = lis.Close()
		_ = os.RemoveAll("./test_grpc_storage")
	}

	return conn, cleanup
}

func TestGRPCHealthService(t *testing.T) {
	conn, cleanup := setupGRPCTestServer(t)
	defer cleanup()

	client := realmv1.NewHealthServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := client.GetHealth(ctx, &realmv1.HealthRequest{})
	if err != nil {
		t.Fatalf("GetHealth failed: %v", err)
	}

	if resp.GetStatus() != "healthy" {
		t.Errorf("Expected status 'healthy', got %q", resp.GetStatus())
	}
	if resp.GetService() != "realm-api" {
		t.Errorf("Expected service 'realm-api', got %q", resp.GetService())
	}
	if resp.GetDatabase() != "disconnected" { // in-memory test without DB
		t.Errorf("Expected database 'disconnected', got %q", resp.GetDatabase())
	}
}

func TestGRPCContactService_Validation(t *testing.T) {
	conn, cleanup := setupGRPCTestServer(t)
	defer cleanup()

	client := realmv1.NewContactServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 1. Test Honeypot (silent success)
	resp, err := client.SendMessage(ctx, &realmv1.SendMessageRequest{
		Name:    "Bot Spammer",
		Email:   "bot@spam.com",
		Subject: "Buy crypto",
		Message: "Check out this amazing coin right now!",
		Gotcha:  "I am a bot",
	})
	if err != nil {
		t.Fatalf("Honeypot request failed: %v", err)
	}
	if resp.GetStatus() != "success" {
		t.Errorf("Expected status 'success' for honeypot, got %q", resp.GetStatus())
	}

	// 2. Test Invalid Name
	_, err = client.SendMessage(ctx, &realmv1.SendMessageRequest{
		Name:    "A",
		Email:   "valid@example.com",
		Subject: "Valid Subject",
		Message: "Valid message body with sufficient length",
	})
	if err == nil {
		t.Error("Expected error for name with < 2 chars, got nil")
	}

	// 3. Test Invalid Email
	_, err = client.SendMessage(ctx, &realmv1.SendMessageRequest{
		Name:    "John Doe",
		Email:   "invalid-email",
		Subject: "Valid Subject",
		Message: "Valid message body with sufficient length",
	})
	if err == nil {
		t.Error("Expected error for invalid email format, got nil")
	}
}

func TestGRPCStorageService_UploadAndInfo(t *testing.T) {
	conn, cleanup := setupGRPCTestServer(t)
	defer cleanup()

	client := realmv1.NewStorageServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Upload a text file payload
	payload := []byte("Hello, this is a gRPC backend storage test payload with enough data to compress.")
	uploadResp, err := client.UploadFile(ctx, &realmv1.UploadFileRequest{
		Filename:    "test.txt",
		ContentType: "text/plain",
		Data:        payload,
	})
	if err != nil {
		t.Fatalf("UploadFile failed: %v", err)
	}

	if uploadResp.GetStatus() != "success" {
		t.Fatalf("Expected upload status 'success', got %q", uploadResp.GetStatus())
	}

	fileMeta := uploadResp.GetFile()
	if fileMeta == nil || fileMeta.GetId() == "" {
		t.Fatalf("Expected valid file metadata, got nil or empty id")
	}
	if fileMeta.GetFilename() != "test.txt" {
		t.Errorf("Expected filename 'test.txt', got %q", fileMeta.GetFilename())
	}
	if fileMeta.GetOriginalSize() != int64(len(payload)) {
		t.Errorf("Expected original size %d, got %d", len(payload), fileMeta.GetOriginalSize())
	}

	// 2. Stream the file back via gRPC GetFile
	stream, err := client.GetFile(ctx, &realmv1.GetFileRequest{
		Id: fileMeta.GetId(),
	})
	if err != nil {
		t.Fatalf("GetFile stream failed: %v", err)
	}

	var downloadedData []byte
	for {
		chunk, err := stream.Recv()
		if err != nil {
			break
		}
		downloadedData = append(downloadedData, chunk.GetChunk()...)
	}

	if string(downloadedData) != string(payload) {
		t.Errorf("Downloaded data mismatch.\nExpected: %s\nGot: %s", string(payload), string(downloadedData))
	}
}
