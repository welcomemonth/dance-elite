package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/meetmonth/dance-elite/internal/model"
)

func testLotteryData() *model.LotteryData {
	return &model.LotteryData{
		Version:   1,
		EventName: "测试赛事抽签",
		Genres:    []model.LotteryGenre{{Key: "classical", Label: "古典舞"}},
		Groups:    []model.LotteryGroup{{Genre: "classical", AgeGroup: "U11", Letter: "A", Size: 2}},
		Players: []model.LotteryPlayer{
			{Name: "张三", IDCard: "310101201501011234", Genre: "classical", AgeGroup: "U11"},
			{Name: "李四", IDCard: "310101201501015678", Genre: "classical", AgeGroup: "U11"},
		},
	}
}

func TestLotteryStoreSaveLoad(t *testing.T) {
	dir := t.TempDir()
	lotteryStore := NewLotteryStore(dir)
	ctx := context.Background()

	if err := lotteryStore.Save(ctx, "Abc12345", testLotteryData()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := lotteryStore.Load(ctx, "Abc12345")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.EventName != "测试赛事抽签" || len(got.Players) != 2 {
		t.Fatalf("unexpected data: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "Abc12345.json")); err != nil {
		t.Fatalf("json file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Abc12345.json.tmp")); !os.IsNotExist(err) {
		t.Fatalf("temp file should be renamed away, err=%v", err)
	}
}

func TestLotteryStoreRejectsInvalidToken(t *testing.T) {
	lotteryStore := NewLotteryStore(t.TempDir())
	ctx := context.Background()
	for _, token := range []string{"", "short", "../escape", "with space", "中文token123"} {
		if _, err := lotteryStore.Load(ctx, token); !errors.Is(err, ErrNotFound) {
			t.Errorf("Load(%q) err = %v, want ErrNotFound", token, err)
		}
		if err := lotteryStore.Save(ctx, token, testLotteryData()); !errors.Is(err, ErrNotFound) {
			t.Errorf("Save(%q) err = %v, want ErrNotFound", token, err)
		}
	}
}

func TestLotteryStoreMutateSerializes(t *testing.T) {
	lotteryStore := NewLotteryStore(t.TempDir())
	ctx := context.Background()
	data := testLotteryData()
	if err := lotteryStore.Save(ctx, "Abc12345", data); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if _, err := lotteryStore.Mutate(ctx, "Abc12345", func(d *model.LotteryData) error {
				d.Version++
				return nil
			}); err != nil {
				t.Errorf("Mutate: %v", err)
			}
		})
	}
	wg.Wait()

	got, err := lotteryStore.Load(ctx, "Abc12345")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Version != 21 {
		t.Fatalf("Version = %d, want 21 (serialized increments)", got.Version)
	}
	if _, err := os.Stat(filepath.Join(lotteryStore.Dir(), "Abc12345.json.bak")); err != nil {
		t.Fatalf("backup file missing: %v", err)
	}
}

func TestLotteryStoreMutateErrorDoesNotWrite(t *testing.T) {
	lotteryStore := NewLotteryStore(t.TempDir())
	ctx := context.Background()
	if err := lotteryStore.Save(ctx, "Abc12345", testLotteryData()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	wantErr := errors.New("stop")
	_, err := lotteryStore.Mutate(ctx, "Abc12345", func(d *model.LotteryData) error {
		d.EventName = "不应写入"
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	got, err := lotteryStore.Load(ctx, "Abc12345")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.EventName != "测试赛事抽签" {
		t.Fatalf("data changed despite error: %q", got.EventName)
	}
}
