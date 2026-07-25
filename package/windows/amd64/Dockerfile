# escape=`
FROM mcr.microsoft.com/windows/nanoserver:ltsc2022

COPY ecr-credential-sync.exe C:\pasturestack\ecr-credential-sync.exe

USER ContainerUser
ENTRYPOINT ["C:\\pasturestack\\ecr-credential-sync.exe"]
