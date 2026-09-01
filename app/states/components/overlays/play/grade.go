package play

import (
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu"
	"github.com/innovationreadytupperware/danser-ee/app/skin"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/batch"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/font"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/texture"
	color2 "github.com/innovationreadytupperware/danser-ee/framework/math/color"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

const lazerFGradeColor = 0x3f3f3f

// GetGradeTexture resolves the legacy texture used for a grade presentation.
// F is intentionally allowed to miss because the legacy skin set has no
// standard F asset.
func GetGradeTexture(grade osu.Grade, small bool) *texture.TextureRegion {
	if grade == osu.NONE {
		return nil
	}

	name := "ranking-" + grade.TextureName()
	if small {
		name += "-small"
	}

	return skin.GetTexture(name)
}

// GradeFallbackColor returns the colour used when a grade must be rendered as
// text instead of a skin texture. The F colour matches lazer's rank colour.
func GradeFallbackColor(grade osu.Grade) color2.Color {
	if grade == osu.F {
		return color2.NewI(lazerFGradeColor)
	}

	return color2.NewL(1)
}

// DrawGradeText renders a grade using the fallback rank presentation. The
// caller controls opacity through the batch colour, just like a sprite draw.
func DrawGradeText(renderer *batch.QuadBatch, fnt *font.Font, grade osu.Grade, position vector.Vector2d, size float64) {
	if fnt == nil || grade == osu.NONE {
		return
	}

	fnt.DrawOriginRotationColor(renderer, position.X, position.Y, vector.Centre, size, 0, false, GradeFallbackColor(grade), grade.String())
}
