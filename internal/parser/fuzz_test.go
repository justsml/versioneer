package parser

import "testing"

func FuzzPackageJSON(f *testing.F) {
	f.Add([]byte(`{"dependencies":{"react":"^18.2.0","next":"14.0.0"},"devDependencies":{"typescript":"^5.3.0"},"peerDependencies":{"react-dom":"^18.0.0"}}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"dependencies":{}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		p := ForFile("package.json")
		p.Parse("package.json", data) // must not panic
	})
}

func FuzzGoMod(f *testing.F) {
	f.Add([]byte("module example.com/foo\n\ngo 1.22\n\nrequire (\n\tgithub.com/stretchr/testify v1.9.0\n\tgolang.org/x/text v0.14.0 // indirect\n)\n\nrequire github.com/single/dep v0.1.0\n"))
	f.Add([]byte("module example.com/foo\n\ngo 1.22\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		p := ForFile("go.mod")
		p.Parse("go.mod", data) // must not panic
	})
}

func FuzzRequirementsTxt(f *testing.F) {
	f.Add([]byte("requests>=2.28.0\nflask==2.3.2\n# comment line\n-r other.txt\nnumpy\nboto3~=1.28\n"))
	f.Add([]byte(""))
	f.Add([]byte("# only comments\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		p := ForFile("requirements.txt")
		p.Parse("requirements.txt", data) // must not panic
	})
}

func FuzzCargoToml(f *testing.F) {
	f.Add([]byte("[package]\nname = \"myapp\"\n\n[dependencies]\nserde = \"1.0\"\ntokio = { version = \"1.35\", features = [\"full\"] }\n\n[dev-dependencies]\ncriterion = \"0.5\"\n"))
	f.Add([]byte("[package]\nname = \"empty\"\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		p := ForFile("Cargo.toml")
		p.Parse("Cargo.toml", data) // must not panic
	})
}

func FuzzPomXML(f *testing.F) {
	f.Add([]byte(`<project><dependencies><dependency><groupId>org.springframework</groupId><artifactId>spring-core</artifactId><version>6.1.0</version></dependency><dependency><groupId>junit</groupId><artifactId>junit</artifactId><version>4.13</version><scope>test</scope></dependency></dependencies></project>`))
	f.Add([]byte(`<project></project>`))
	f.Add([]byte(`<project><dependencies></dependencies></project>`))
	f.Fuzz(func(t *testing.T, data []byte) {
		p := ForFile("pom.xml")
		p.Parse("pom.xml", data) // must not panic
	})
}

func FuzzComposerJSON(f *testing.F) {
	f.Add([]byte(`{"require":{"php":"^8.1","laravel/framework":"^10.0","ext-json":"*"},"require-dev":{"phpunit/phpunit":"^10.0"}}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"require":{}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		p := ForFile("composer.json")
		p.Parse("composer.json", data) // must not panic
	})
}
