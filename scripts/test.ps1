param()

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$goTestArguments = @($args)

if ($goTestArguments.Count -eq 0) {
	$goTestArguments = @('./...')
}

# Go executes test binaries from a temporary build directory. Native runtime
# files live in the repository root, so add that directory to the process
# search path before the test binary is launched.
$pathSeparator = [System.IO.Path]::PathSeparator
$env:PATH = $repoRoot + $pathSeparator + $env:PATH

# Some managed Windows installations deny writes to Go's default per-user
# build cache. Keep the test wrapper self-contained when no cache was chosen
# explicitly, and rely on the repository's ignored cache directory instead.
if ([string]::IsNullOrWhiteSpace($env:GOCACHE)) {
	$env:GOCACHE = Join-Path $repoRoot 'cache/go-build'
}

if ($IsLinux) {
	$env:LD_LIBRARY_PATH = $repoRoot + ':' + $env:LD_LIBRARY_PATH
} elseif ($IsMacOS) {
	$env:DYLD_LIBRARY_PATH = $repoRoot + ':' + $env:DYLD_LIBRARY_PATH
}

Push-Location -LiteralPath $repoRoot
$exitCode = 1
try {
	& go test @goTestArguments
	$exitCode = $LASTEXITCODE
} finally {
	Pop-Location
}

exit $exitCode
