param(
	# Optional explicit fixtures directory. Defaults to <repo>/temp.
	[string]$Fixtures,
	# Optional -run pattern forwarded to the test binary.
	[string]$Run = 'TestReplayHeaderConformance'
)

# Replay conformance replays .osr files through the judgement path and compares
# the result against the judgement counts osu!stable stored in each replay
# header.
#
# The simulation needs the bundled assets and SDL3.dll, which env.LibDir
# resolves relative to the test binary's directory, so the binary is built into
# the repository root and run from there rather than through a plain `go test`.

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path

$pathSeparator = [System.IO.Path]::PathSeparator
$env:PATH = $repoRoot + $pathSeparator + $env:PATH

if ([string]::IsNullOrWhiteSpace($env:GOCACHE)) {
	$env:GOCACHE = Join-Path $repoRoot 'cache/go-build'
}

if ([string]::IsNullOrWhiteSpace($Fixtures)) {
	$Fixtures = Join-Path $repoRoot 'temp'
}
$env:DANSER_REPLAY_FIXTURES = (Resolve-Path -LiteralPath $Fixtures -ErrorAction SilentlyContinue) ?? $Fixtures

$binary = Join-Path $repoRoot 'replayconformance.test.exe'

Push-Location -LiteralPath $repoRoot
$exitCode = 1
try {
	$buildArgs = @('test', '-c', '-o', $binary)
	if (-not [string]::IsNullOrWhiteSpace($env:CC)) {
		$buildArgs += @('-ldflags', "-extld=$env:CC")
	}
	$buildArgs += './app/rulesets/osu/'
	& go @buildArgs
	if ($LASTEXITCODE -ne 0) {
		exit $LASTEXITCODE
	}

	& $binary "-test.run=$Run" "-test.v=true"
	$exitCode = $LASTEXITCODE
} finally {
	Remove-Item -LiteralPath $binary -ErrorAction SilentlyContinue
	Pop-Location
}

exit $exitCode
