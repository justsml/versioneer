package parser

import (
	"testing"

	"github.com/justsml/versioneer/internal/model"
)

func assertDep(t *testing.T, d model.Dependency, name, version, depType string) {
	t.Helper()
	if d.Name != name {
		t.Errorf("name: got %q, want %q", d.Name, name)
	}
	if d.Version != version {
		t.Errorf("version: got %q, want %q", d.Version, version)
	}
	if d.DepType != depType {
		t.Errorf("depType: got %q, want %q", d.DepType, depType)
	}
}

func TestGoMod(t *testing.T) {
	data := []byte(`module example.com/foo

go 1.22

require (
	github.com/stretchr/testify v1.9.0
	golang.org/x/text v0.14.0 // indirect
)

require github.com/single/dep v0.1.0
`)
	deps, err := goMod{}.Parse("go.mod", data)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 3 {
		t.Fatalf("expected 3 deps, got %d", len(deps))
	}
	assertDep(t, deps[0], "github.com/stretchr/testify", "v1.9.0", "direct")
	assertDep(t, deps[1], "golang.org/x/text", "v0.14.0", "indirect")
	assertDep(t, deps[2], "github.com/single/dep", "v0.1.0", "direct")
}

func TestPackageJSON(t *testing.T) {
	data := []byte(`{
  "dependencies": {"react": "^18.2.0", "next": "14.0.0"},
  "devDependencies": {"typescript": "^5.3.0"},
  "peerDependencies": {"react-dom": "^18.0.0"}
}`)
	deps, err := packageJSON{}.Parse("package.json", data)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 4 {
		t.Fatalf("expected 4 deps, got %d", len(deps))
	}
	found := map[string]string{}
	for _, d := range deps {
		found[d.Name] = d.DepType
	}
	if found["react"] != "direct" {
		t.Error("react should be direct")
	}
	if found["typescript"] != "dev" {
		t.Error("typescript should be dev")
	}
	if found["react-dom"] != "peer" {
		t.Error("react-dom should be peer")
	}
}

func TestRequirementsTxt(t *testing.T) {
	data := []byte(`
requests>=2.28.0
flask==2.3.2
# comment line
-r other.txt
numpy
boto3~=1.28
`)
	deps, err := requirementsTxt{}.Parse("requirements.txt", data)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 4 {
		t.Fatalf("expected 4 deps, got %d: %+v", len(deps), deps)
	}
}

func TestCargoToml(t *testing.T) {
	data := []byte(`
[package]
name = "myapp"

[dependencies]
serde = "1.0"
tokio = { version = "1.35", features = ["full"] }

[dev-dependencies]
criterion = "0.5"
`)
	deps, err := cargoToml{}.Parse("Cargo.toml", data)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 3 {
		t.Fatalf("expected 3 deps, got %d", len(deps))
	}
	assertDep(t, deps[0], "serde", "1.0", "direct")
	assertDep(t, deps[1], "tokio", "1.35", "direct")
	assertDep(t, deps[2], "criterion", "0.5", "dev")
}

func TestPomXML(t *testing.T) {
	data := []byte(`<project>
  <dependencies>
    <dependency>
      <groupId>org.springframework</groupId>
      <artifactId>spring-core</artifactId>
      <version>6.1.0</version>
    </dependency>
    <dependency>
      <groupId>junit</groupId>
      <artifactId>junit</artifactId>
      <version>4.13</version>
      <scope>test</scope>
    </dependency>
  </dependencies>
</project>`)
	deps, err := pomXML{}.Parse("pom.xml", data)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 2 {
		t.Fatalf("expected 2 deps, got %d", len(deps))
	}
	assertDep(t, deps[0], "org.springframework:spring-core", "6.1.0", "direct")
	assertDep(t, deps[1], "junit:junit", "4.13", "dev")
}

func TestGemfile(t *testing.T) {
	data := []byte(`
source 'https://rubygems.org'
gem 'rails', '~> 7.0'
gem 'puma'

group :development, :test do
  gem 'rspec', '~> 3.12'
end
`)
	deps, err := gemfile{}.Parse("Gemfile", data)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 3 {
		t.Fatalf("expected 3 deps, got %d", len(deps))
	}
	assertDep(t, deps[0], "rails", "~> 7.0", "direct")
	assertDep(t, deps[1], "puma", "", "direct")
	assertDep(t, deps[2], "rspec", "~> 3.12", "dev")
}

func TestComposerJSON(t *testing.T) {
	data := []byte(`{
  "require": {"php": "^8.1", "laravel/framework": "^10.0", "ext-json": "*"},
  "require-dev": {"phpunit/phpunit": "^10.0"}
}`)
	deps, err := composerJSON{}.Parse("composer.json", data)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 2 {
		t.Fatalf("expected 2 deps (php and ext-* filtered), got %d: %+v", len(deps), deps)
	}
	assertDep(t, deps[0], "laravel/framework", "^10.0", "direct")
	assertDep(t, deps[1], "phpunit/phpunit", "^10.0", "dev")
}

func TestGradle(t *testing.T) {
	data := []byte(`
plugins {
    id 'java'
}
dependencies {
    implementation 'org.springframework:spring-core:6.1.0'
    testImplementation 'junit:junit:4.13'
    api 'com.google.guava:guava:32.0'
}
`)
	deps, err := gradle{}.Parse("build.gradle", data)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 3 {
		t.Fatalf("expected 3 deps, got %d", len(deps))
	}
	assertDep(t, deps[0], "org.springframework:spring-core", "6.1.0", "direct")
	assertDep(t, deps[1], "junit:junit", "4.13", "dev")
	assertDep(t, deps[2], "com.google.guava:guava", "32.0", "direct")
}
