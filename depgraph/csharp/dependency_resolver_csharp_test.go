package csharp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveCSharpProjectImports(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "App"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "Lib"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "Test.csproj"), []byte(`<Project Sdk="Microsoft.NET.Sdk"></Project>`), 0o644))

	programPath := filepath.Join(tmpDir, "App", "Program.cs")
	require.NoError(t, os.WriteFile(programPath, []byte(`using Lib.Core;

namespace App;

public class Program
{
    private Logger logger;
    public void Run()
    {
        var fileLogger = new FileLogger("log.txt");
    }
}
`), 0o644))

	loggerPath := filepath.Join(tmpDir, "Lib", "Logger.cs")
	require.NoError(t, os.WriteFile(loggerPath, []byte(`namespace Lib.Core;
public class Logger {}
`), 0o644))

	fileLoggerPath := filepath.Join(tmpDir, "Lib", "FileLogger.cs")
	require.NoError(t, os.WriteFile(fileLoggerPath, []byte(`namespace Lib.Core;
public class FileLogger {}
`), 0o644))

	helperPath := filepath.Join(tmpDir, "App", "Helper.cs")
	require.NoError(t, os.WriteFile(helperPath, []byte(`namespace App;
public class Helper {}
`), 0o644))

	supplied := map[string]bool{
		programPath:    true,
		loggerPath:     true,
		fileLoggerPath: true,
		helperPath:     true,
	}
	reader := vcs.FilesystemContentReader()
	namespaceToFiles, namespaceToTypes, fileToNamespace, fileToScope := BuildCSharpIndices(supplied, reader)

	imports, err := ResolveCSharpProjectImports(
		programPath,
		programPath,
		namespaceToFiles,
		namespaceToTypes,
		fileToNamespace,
		fileToScope,
		supplied,
		reader)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{loggerPath, fileLoggerPath}, imports)
}

func TestResolveCSharpProjectImports_SkipsAmbiguousType(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "A"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "B"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "Test.csproj"), []byte(`<Project Sdk="Microsoft.NET.Sdk"></Project>`), 0o644))

	programPath := filepath.Join(tmpDir, "Program.cs")
	require.NoError(t, os.WriteFile(programPath, []byte(`using Shared;
public class Program { private Service svc; }
`), 0o644))

	serviceAPath := filepath.Join(tmpDir, "A", "Service.cs")
	require.NoError(t, os.WriteFile(serviceAPath, []byte(`namespace Shared;
public class Service {}
`), 0o644))

	serviceBPath := filepath.Join(tmpDir, "B", "Service.cs")
	require.NoError(t, os.WriteFile(serviceBPath, []byte(`namespace Shared;
public class Service {}
`), 0o644))

	supplied := map[string]bool{
		programPath:  true,
		serviceAPath: true,
		serviceBPath: true,
	}
	reader := vcs.FilesystemContentReader()
	namespaceToFiles, namespaceToTypes, fileToNamespace, fileToScope := BuildCSharpIndices(supplied, reader)

	imports, err := ResolveCSharpProjectImports(
		programPath,
		programPath,
		namespaceToFiles,
		namespaceToTypes,
		fileToNamespace,
		fileToScope,
		supplied,
		reader)
	require.NoError(t, err)
	assert.Empty(t, imports)
}

func TestResolveCSharpProjectImports_DoesNotLinkMethodNameToConcreteType(t *testing.T) {
	tmpDir := t.TempDir()

	monsterPath := filepath.Join(tmpDir, "MonsterGrain.cs")
	require.NoError(t, os.WriteFile(monsterPath, []byte(`using AdventureGrainInterfaces;

namespace AdventureGrains;

public class MonsterGrain
{
    private IRoomGrain? _roomGrain;

    Task<IRoomGrain> RoomGrain() => Task.FromResult(_roomGrain!);
}
`), 0o644))

	roomPath := filepath.Join(tmpDir, "RoomGrain.cs")
	require.NoError(t, os.WriteFile(roomPath, []byte(`namespace AdventureGrains;
public class RoomGrain {}
`), 0o644))

	iRoomPath := filepath.Join(tmpDir, "IRoomGrain.cs")
	require.NoError(t, os.WriteFile(iRoomPath, []byte(`namespace AdventureGrainInterfaces;
public interface IRoomGrain {}
`), 0o644))

	supplied := map[string]bool{
		monsterPath: true,
		roomPath:    true,
		iRoomPath:   true,
	}
	reader := vcs.FilesystemContentReader()
	namespaceToFiles, namespaceToTypes, fileToNamespace, fileToScope := BuildCSharpIndices(supplied, reader)

	imports, err := ResolveCSharpProjectImports(
		monsterPath,
		monsterPath,
		namespaceToFiles,
		namespaceToTypes,
		fileToNamespace,
		fileToScope,
		supplied,
		reader)
	require.NoError(t, err)
	assert.Contains(t, imports, iRoomPath)
	assert.NotContains(t, imports, roomPath)
}

func TestResolveCSharpProjectImports_SeparatesDuplicateTypesByProjectScope(t *testing.T) {
	tmpDir := t.TempDir()

	startDir := filepath.Join(tmpDir, "start")
	finishedDir := filepath.Join(tmpDir, "finished")
	require.NoError(t, os.MkdirAll(startDir, 0o755))
	require.NoError(t, os.MkdirAll(finishedDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(startDir, "start.csproj"), []byte(`<Project Sdk="Microsoft.NET.Sdk"></Project>`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(finishedDir, "finished.csproj"), []byte(`<Project Sdk="Microsoft.NET.Sdk"></Project>`), 0o644))

	startProgramPath := filepath.Join(startDir, "Program.cs")
	require.NoError(t, os.WriteFile(startProgramPath, []byte(`using Calculators;
using ConsumerVehicleRegistration;
public class Program { void Run(Car c) { _ = TollCalculator.CalculateToll(c); } }
`), 0o644))
	startCalculatorPath := filepath.Join(startDir, "TollCalculator.cs")
	require.NoError(t, os.WriteFile(startCalculatorPath, []byte(`namespace Calculators;
public static class TollCalculator { public static decimal CalculateToll(object vehicle) => 0; }
`), 0o644))
	startExternalPath := filepath.Join(startDir, "ExternalSystems.cs")
	require.NoError(t, os.WriteFile(startExternalPath, []byte(`namespace ConsumerVehicleRegistration;
public class Car { public int Passengers { get; set; } }
`), 0o644))

	finishedCalculatorPath := filepath.Join(finishedDir, "TollCalculator.cs")
	require.NoError(t, os.WriteFile(finishedCalculatorPath, []byte(`namespace Calculators;
public static class TollCalculator { public static decimal CalculateToll(object vehicle) => 1; }
`), 0o644))
	finishedExternalPath := filepath.Join(finishedDir, "ExternalSystems.cs")
	require.NoError(t, os.WriteFile(finishedExternalPath, []byte(`namespace ConsumerVehicleRegistration;
public class Car { public int Passengers { get; set; } }
`), 0o644))

	supplied := map[string]bool{
		startProgramPath:       true,
		startCalculatorPath:    true,
		startExternalPath:      true,
		finishedCalculatorPath: true,
		finishedExternalPath:   true,
	}
	reader := vcs.FilesystemContentReader()
	namespaceToFiles, namespaceToTypes, fileToNamespace, fileToScope := BuildCSharpIndices(supplied, reader)

	imports, err := ResolveCSharpProjectImports(
		startProgramPath,
		startProgramPath,
		namespaceToFiles,
		namespaceToTypes,
		fileToNamespace,
		fileToScope,
		supplied,
		reader)
	require.NoError(t, err)
	assert.Contains(t, imports, startCalculatorPath)
	assert.Contains(t, imports, startExternalPath)
	assert.NotContains(t, imports, finishedCalculatorPath)
	assert.NotContains(t, imports, finishedExternalPath)
}

func TestResolveCSharpProjectImports_LinksUniqueCrossScopeType(t *testing.T) {
	tmpDir := t.TempDir()

	projADir := filepath.Join(tmpDir, "A")
	projBDir := filepath.Join(tmpDir, "B")
	require.NoError(t, os.MkdirAll(projADir, 0o755))
	require.NoError(t, os.MkdirAll(projBDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projADir, "a.csproj"), []byte(`<Project Sdk="Microsoft.NET.Sdk"></Project>`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(projBDir, "b.csproj"), []byte(`<Project Sdk="Microsoft.NET.Sdk"></Project>`), 0o644))

	sourcePath := filepath.Join(projADir, "Source.cs")
	require.NoError(t, os.WriteFile(sourcePath, []byte(`namespace A;
public class Source { public SharedType Value { get; set; } = null!; }
`), 0o644))
	targetPath := filepath.Join(projBDir, "SharedType.cs")
	require.NoError(t, os.WriteFile(targetPath, []byte(`namespace B;
public class SharedType {}
`), 0o644))

	supplied := map[string]bool{
		sourcePath: true,
		targetPath: true,
	}
	reader := vcs.FilesystemContentReader()
	namespaceToFiles, namespaceToTypes, fileToNamespace, fileToScope := BuildCSharpIndices(supplied, reader)

	imports, err := ResolveCSharpProjectImports(
		sourcePath,
		sourcePath,
		namespaceToFiles,
		namespaceToTypes,
		fileToNamespace,
		fileToScope,
		supplied,
		reader)
	require.NoError(t, err)
	assert.Contains(t, imports, targetPath)
}
