@echo off
echo Starting PubSub Server with Docker Compose...
echo.

REM Check if Docker is running
docker info >nul 2>&1
if errorlevel 1 (
    echo ERROR: Docker Desktop is not running!
    echo Please start Docker Desktop and try again.
    pause
    exit /b 1
)

echo Docker Desktop is running...
echo.

REM Check if .env file exists
if not exist .env (
    echo WARNING: .env file not found!
    echo Creating .env from .env.example...
    copy .env.example .env
    echo.
    echo IMPORTANT: Please edit .env and set DB_PASSWORD!
    echo Press any key after editing .env file...
    pause
)

echo Starting services...
docker-compose up -d

if errorlevel 1 (
    echo.
    echo ERROR: Failed to start services!
    pause
    exit /b 1
)

echo.
echo ======================================
echo PubSub Server started successfully!
echo ======================================
echo.
echo MySQL:          localhost:3306
echo PubSub Server:  http://localhost:8080
echo.
echo API Endpoints:
echo   POST   http://localhost:8080/api/v1/publish
echo   POST   http://localhost:8080/api/v1/subscribe
echo   GET    http://localhost:8080/api/v1/subscriptions
echo   DELETE http://localhost:8080/api/v1/subscriptions/:id
echo   GET    http://localhost:8080/api/v1/health
echo.
echo Useful commands:
echo   docker-compose logs -f          - View logs
echo   docker-compose ps               - Check status
echo   docker-compose down             - Stop services
echo   docker-compose down -v          - Stop and remove volumes
echo.
echo Press any key to view logs...
pause >nul
docker-compose logs -f
