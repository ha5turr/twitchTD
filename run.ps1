# run.ps1 keeps the game server running on the streaming PC.
#
#   powershell -ExecutionPolicy Bypass -File .\run.ps1 [twitchtd flags...]
#
# - Starts twitchtd.exe from this folder (config.json and twitchtd.db live
#   here too). If there is no exe yet, builds one with Go.
# - Exit code 3 means !tdupdate or !tdrollback left twitchtd.next.exe:
#   the current build is kept as twitchtd.prev.exe and the new one starts.
# - If a freshly swapped build dies within 30 seconds, the previous one is
#   put back; the game then tells the chat the update did not start.
# - A crash restarts the server after 3 seconds; Ctrl+C (exit code 0) stops.
# - The console's QuickEdit is switched off: a click into the window would
#   otherwise freeze anything printing to it, the server and this script
#   alike, until a key is pressed (an update then seemed to hang).

$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot
$env:TWITCHTD_SUPERVISED = '1'

try {
    Add-Type -Namespace TwitchTD -Name Console -MemberDefinition @'
[DllImport("kernel32.dll")] public static extern IntPtr GetStdHandle(int handle);
[DllImport("kernel32.dll")] public static extern bool GetConsoleMode(IntPtr handle, out uint mode);
[DllImport("kernel32.dll")] public static extern bool SetConsoleMode(IntPtr handle, uint mode);
'@
    $stdin = [TwitchTD.Console]::GetStdHandle(-10)
    $mode = [uint32]0
    if ([TwitchTD.Console]::GetConsoleMode($stdin, [ref]$mode)) {
        # Clear ENABLE_QUICK_EDIT_MODE (0x40); ENABLE_EXTENDED_FLAGS (0x80) makes it stick.
        [void][TwitchTD.Console]::SetConsoleMode($stdin, [uint32](($mode -band 0xFFFFFFBF) -bor 0x80))
    }
} catch {
    Write-Host "run.ps1: could not switch QuickEdit off ($_), do not click into this window"
}

$exe = Join-Path $PSScriptRoot 'twitchtd.exe'
$next = Join-Path $PSScriptRoot 'twitchtd.next.exe'
$prev = Join-Path $PSScriptRoot 'twitchtd.prev.exe'

if (-not (Test-Path $exe)) {
    Write-Host 'twitchtd.exe not found, building it with Go...'
    go build -o $exe ./cmd/twitchtd
    if ($LASTEXITCODE -ne 0) { throw 'go build failed' }
}

$swapped = $false
while ($true) {
    if (Test-Path $next) {
        Copy-Item $exe $prev -Force
        Move-Item $next $exe -Force
        $swapped = $true
        Write-Host 'run.ps1: switched to the new build'
    }
    $started = Get-Date
    try {
        & $exe @args
        $code = $LASTEXITCODE
    } catch {
        Write-Host "run.ps1: could not start twitchtd.exe: $_"
        $code = -1 # a broken download: treated like a crash at once
    }
    $uptime = ((Get-Date) - $started).TotalSeconds

    if ($code -eq 0) { break } # stopped by hand
    if ($code -eq 3) { continue } # update or rollback: swap and start again

    if ($swapped -and $uptime -lt 30 -and (Test-Path $prev)) {
        Write-Host "run.ps1: the new build exited with $code after $([int]$uptime) s, putting the previous one back"
        Copy-Item $prev $exe -Force
    } else {
        Write-Host "run.ps1: twitchtd exited with $code, restarting in 3 s"
        Start-Sleep -Seconds 3
    }
    $swapped = $false
}
