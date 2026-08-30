package containers

import (
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

func TestRemoveExpiredRenderablesBeforeDrawing(t *testing.T) {
	expiredObject := objects.DummyCircle(vector.NewVec2f(0, 0), 0)
	activeObject := objects.DummyCircle(vector.NewVec2f(0, 0), 1)

	container := &HitObjectContainer{
		renderables: []*renderableProxy{
			{
				renderable:   expiredObject,
				isSliderBody: false,
				endTime:      100,
			},
			{
				renderable:   activeObject,
				isSliderBody: false,
				endTime:      200,
			},
			{
				renderable:   expiredObject,
				isSliderBody: true,
				endTime:      100,
			},
		},
		countProcessed: 2,
	}

	container.removeExpiredRenderables(100)

	if len(container.renderables) != 1 || container.renderables[0].renderable != activeObject {
		t.Fatalf("active renderables after cleanup = %v, want one active object", container.renderables)
	}

	if container.countProcessed != 1 {
		t.Fatalf("processed object count after cleanup = %d, want 1", container.countProcessed)
	}
}
