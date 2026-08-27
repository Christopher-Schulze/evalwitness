package product

import "testing"

func TestVersionIsCanonical(t *testing.T) {
	if !ValidVersion(Version) {
		t.Fatalf("Version %q is not canonical SemVer", Version)
	}
}

func TestValidVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "release", value: "1.2.3", want: true},
		{name: "prerelease", value: "1.2.3-rc.1", want: true},
		{name: "prerelease hyphen", value: "1.2.3-alpha-beta", want: true},
		{name: "build", value: "1.2.3+build.01", want: true},
		{name: "complete", value: "1.2.3-rc.1+build.01", want: true},
		{name: "tag prefix", value: "v1.2.3", want: false},
		{name: "leading zero", value: "01.2.3", want: false},
		{name: "prerelease leading zero", value: "1.2.3-01", want: false},
		{name: "missing patch", value: "1.2", want: false},
		{name: "empty identifier", value: "1.2.3-rc..1", want: false},
		{name: "second build delimiter", value: "1.2.3+a+b", want: false},
		{name: "space", value: "1.2.3 rc", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := ValidVersion(test.value); got != test.want {
				t.Fatalf("ValidVersion(%q) = %t, want %t", test.value, got, test.want)
			}
		})
	}
}
