# Burn Oakridge image into 3rd party devices
1. convert/... convert 3rd party to Oakridge OS
2. restore/... restore back to 3rd party factory image

Both main.go has the main logic, other directory is tool libaray

# Usage:
Executable file name is: ``convert.linux``, ``convert.mac`` and ``convert.exe``(on Windows).
1. Linux/MacOS
    Make sure downloaded file is executable, here are examples on Linux:
    ```
    ./convert.linux
    ```
    This will scan all subnet the current Linux machine is on
    ```
    ./convert.linux 10.1.1.0/24
    ```
    This will scan subnet 10.1.1.0/24
    ```
    ./convert.linux 10.1.1.123/32
    ```
    This will only try 10.1.1.123

2. Windows
    Double click ``convert.exe`` will scan all subnet the current Windows machine is on.
    Or open a ``cmd`` window, execute like Linux/MacOS to scan for specific subnet/host
