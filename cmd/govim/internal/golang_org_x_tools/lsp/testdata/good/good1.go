package good //@diag("package", "", "")

import (
	"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/lsp/types" //@item(types_import, "types", "\"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/lsp/types\"", "package")
)

func random() int { //@item(good_random, "random", "func() int", "func")
	y := 6 + 7
	return y
}

func random2(y int) int { //@item(good_random2, "random2", "func(y int) int", "func"),item(good_y_param, "y", "int", "parameter")
	//@complete("", good_y_param, types_import, good_random, good_random2, good_stuff)
	var b types.Bob = &types.X{}
	if _, ok := b.(*types.X); ok { //@complete("X", X_struct, Y_struct, Bob_interface)
	}

	return y
}
