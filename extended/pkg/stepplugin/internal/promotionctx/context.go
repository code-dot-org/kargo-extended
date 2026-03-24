package promotionctx

import (
	"os"
	"path/filepath"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	"github.com/akuity/kargo/pkg/promotion"
)

func NewBuiltinPromotionContext(
	promo *kargoapi.Promotion,
	stage *kargoapi.Stage,
	actor string,
	uiBaseURL string,
) (promotion.Context, bool, error) {
	workDir := filepath.Join(os.TempDir(), "promotion-"+string(promo.UID))
	freshWorkDir := false
	err := os.Mkdir(workDir, 0o700)
	switch {
	case err == nil:
		freshWorkDir = true
	case !os.IsExist(err):
		return promotion.Context{}, false, err
	}

	return promotion.NewContext(
		promo,
		stage,
		promotion.WithActor(actor),
		promotion.WithUIBaseURL(uiBaseURL),
		promotion.WithWorkDir(workDir),
	), freshWorkDir, nil
}
