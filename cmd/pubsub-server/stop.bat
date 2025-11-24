@echo off
echo Stopping PubSub Server...
echo.

docker-compose down

echo.
echo Services stopped.
echo.
echo To remove all data (including database):
echo   docker-compose down -v
echo.
pause
