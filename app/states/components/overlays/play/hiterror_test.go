package play

import (
	"testing"

	"github.com/wieku/danser-go/framework/graphics/texture"
	color2 "github.com/wieku/danser-go/framework/math/color"
	"github.com/wieku/danser-go/framework/math/vector"
)

func TestHitErrorJudgmentLinePoolCapsActiveLines(t *testing.T) {
	pixel := texture.TextureRegion{}
	pool := newHitErrorJudgmentLinePool(&pixel)

	for i := 0; i < maxHitErrorJudgmentLines+7; i++ {
		pool.add(float64(i), 10000, float64(i), vector.NewVec2d(float64(i), 0), 1, 3, 1, 0.4, color2.NewL(1), true)
	}

	active := 0
	for _, line := range pool.lines {
		if line.active {
			active++
		}
	}

	if active != maxHitErrorJudgmentLines {
		t.Fatalf("active judgment lines = %d, want %d", active, maxHitErrorJudgmentLines)
	}
}

func TestHitErrorJudgmentLinePoolExpiresLinesAtTheirEndTime(t *testing.T) {
	pixel := texture.TextureRegion{}
	pool := newHitErrorJudgmentLinePool(&pixel)

	pool.add(100, 50, 0, vector.NewVec2d(0, 0), 1, 3, 1, 0.4, color2.NewL(1), true)
	pool.update(149)
	if !pool.lines[0].active {
		t.Fatal("judgment line expired before its end time")
	}

	pool.update(150)
	if pool.lines[0].active {
		t.Fatal("judgment line remained active at its end time")
	}
}
