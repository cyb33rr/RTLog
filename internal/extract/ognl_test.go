package extract

import "testing"

func TestExtractTargetsOGNLFalsePositive(t *testing.T) {
	// Struts2 S2-032 exploit with OGNL class references in URL-encoded payload
	cmd := `curl -sk -D - "https://target.com/action?method:%23_memberAccess%3d@ognl.OgnlContext@DEFAULT_MEMBER_ACCESS,%23a%3d@java.lang.Runtime@getRuntime(),%23out%3d@org.apache.struts2.ServletActionContext@getResponse()" 2>/dev/null`
	result := ExtractTargets(cmd, "curl")
	
	// These should NOT be extracted as hosts
	falsePositives := []string{
		"ognl.ognlcontext",
		"java.lang.runtime",
		"org.apache.struts2.servletactioncontext",
	}
	for _, fp := range falsePositives {
		if result.Hosts.Has(fp) {
			t.Errorf("should not extract OGNL class reference %q as host", fp)
		}
	}
	
	// The actual target URL host SHOULD be extracted
	if !result.Hosts.Has("target.com") {
		t.Errorf("expected host target.com, got %v", result.Hosts)
	}
}
