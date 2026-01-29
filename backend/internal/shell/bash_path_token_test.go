package shell

import "testing"

func TestIsPlainPathToken_AllowsUnicodeLettersAndNumbers(t *testing.T) {
	token := "/tmp/泰瑞沙_NSCLC_品牌计划包_FY26/00_Mock数据清单_Mock_Data_Inventory.md"
	if !isPlainPathToken(token) {
		t.Fatalf("expected token to be treated as plain path token: %q", token)
	}
}

