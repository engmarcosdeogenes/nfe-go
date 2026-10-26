package mdfe

import (
	"os"
	"os/exec"
	"testing"
)

// pythonLxml acha um python com lxml: NFE_PYTHON, python3, python. Falha (não pula)
// se não houver, porque sem validação XSD o teste não prova nada.
func pythonLxml(t *testing.T) string {
	t.Helper()
	for _, c := range []string{os.Getenv("NFE_PYTHON"), "python3", "python"} {
		if c == "" {
			continue
		}
		if p, err := exec.LookPath(c); err == nil && exec.Command(p, "-c", "import lxml").Run() == nil {
			return p
		}
	}
	t.Fatal("instale python3 + lxml ou defina NFE_PYTHON")
	return ""
}
