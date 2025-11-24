@echo off
echo Testing PubSub Server API...
echo.

REM Check if server is running
curl -s http://localhost:8080/api/v1/health >nul 2>&1
if errorlevel 1 (
    echo ERROR: Server is not running!
    echo Run start.bat first.
    pause
    exit /b 1
)

echo 1. Testing Health Check...
curl -X GET http://localhost:8080/api/v1/health
echo.
echo.

echo 2. Testing Publish (will fail - no topics/subscribers yet)...
curl -X POST http://localhost:8080/api/v1/publish ^
  -H "Content-Type: application/json" ^
  -d "{\"topicCode\":\"test.topic\",\"identifier\":\"test-123\",\"data\":{\"message\":\"Hello PubSub!\"}}"
echo.
echo.

echo 3. Testing List Subscriptions...
curl -X GET http://localhost:8080/api/v1/subscriptions
echo.
echo.

echo NOTE: To fully test the API, you need to:
echo   1. Run database migrations
echo   2. Create topics, subscribers, and subscriptions
echo   3. Then publish messages
echo.
pause
