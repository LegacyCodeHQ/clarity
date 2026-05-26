package registry

import (
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/c"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/cpp"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/csharp"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/dart"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/golang"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/java"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/javascript"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/kotlin"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/markdown"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/python"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/ruby"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/rust"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/scala"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/svelte"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/swift"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/typescript"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/zig"
)

var modules = []Module{
	c.Module{},
	cpp.Module{},
	csharp.Module{},
	dart.Module{},
	golang.Module{},
	javascript.Module{},
	java.Module{},
	kotlin.Module{},
	markdown.Module{},
	python.Module{},
	ruby.Module{},
	rust.Module{},
	scala.Module{},
	svelte.Module{},
	swift.Module{},
	typescript.Module{},
	zig.Module{},
}

// Modules returns supported language modules in deterministic order.
func Modules() []Module {
	return append([]Module(nil), modules...)
}

// ModuleForExtension returns the module registered for the provided extension.
func ModuleForExtension(ext string) (Module, bool) {
	for _, module := range modules {
		for _, moduleExt := range module.Extensions() {
			if moduleExt == ext {
				return module, true
			}
		}
	}

	return nil, false
}
