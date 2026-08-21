package test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/auth"
	"github.com/irvanmalik48/realm-api/internal/model"
	"github.com/irvanmalik48/realm-api/internal/service"
	"github.com/irvanmalik48/realm-api/internal/storage"
)

func BenchmarkPASETO_GenerateAndVerify(b *testing.B) {
	pasetoSvc, err := auth.NewPasetoService("707172737475767778797a7b7c7d7e7f808182838485868788898a8b8c8d8e8f")
	if err != nil {
		b.Fatalf("failed to init paseto: %v", err)
	}

	avatar := "https://example.com/avatar.png"
	user := &model.User{
		ID:        uuid.New(),
		Email:     "benchmark@example.com",
		Username:  "benchuser",
		FullName:  "Benchmark User",
		AvatarURL: &avatar,
		Provider:  "local",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		token, err := pasetoSvc.GenerateToken(user, 1*time.Hour)
		if err != nil {
			b.Fatalf("generate token failed: %v", err)
		}

		claims, err := pasetoSvc.VerifyToken(token)
		if err != nil || claims == nil {
			b.Fatalf("verify token failed: %v", err)
		}
	}
}

func BenchmarkToken_HashAndVerify(b *testing.B) {
	tokenSvc := service.NewTokenService(nil, nil, nil)
	rawToken := "rlm_live_0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = tokenSvc.HashToken(rawToken)
	}
}

func BenchmarkStorage_ProcessImage(b *testing.B) {
	// Generate a 100x100 PNG test image
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		b.Fatalf("failed to encode png: %v", err)
	}
	pngData := buf.Bytes()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		info, err := storage.ProcessImage(bytes.NewReader(pngData))
		if err != nil || info == nil {
			b.Fatalf("process image failed: %v", err)
		}
	}
}

func BenchmarkAuth_CheckAvailability(b *testing.B) {
	repo := newMockUserRepo()
	pasetoSvc, _ := auth.NewPasetoService("707172737475767778797a7b7c7d7e7f808182838485868788898a8b8c8d8e8f")
	authSvc := service.NewAuthService(repo, pasetoSvc)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := authSvc.CheckAvailability(ctx, "benchuser", "bench@example.com")
		if err != nil {
			b.Fatalf("check availability failed: %v", err)
		}
	}
}
