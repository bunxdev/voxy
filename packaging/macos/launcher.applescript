on run
  set launcher to POSIX path of (path to resource "Voxy.command")
  do shell script "/usr/bin/open -a Terminal " & quoted form of launcher
end run
