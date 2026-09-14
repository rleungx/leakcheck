package ordinary_main_no_import

import tt "testing"

func TestMain(t *tt.T) {} // want "test function TestMain is not covered by goleak"
