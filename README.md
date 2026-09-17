# Live Polling Tool

A live polling application built for the GUVI Developer Internship task.

## Stack
- Frontend: React + Vite
- Backend: Go + Gin
- Database: MongoDB
- Realtime: Redis Pub/Sub + WebSocket

## Project structure
```text
live-polling-tool/
├── frontend/
└── backend/
```

## 1. Backend

Requirements:
- Go 1.22+
- MongoDB
- Redis

Create `backend/.env` from the example:

```env
PORT=8080
MONGO_URI=mongodb://localhost:27017
MONGO_DB=polling_app
REDIS_ADDR=localhost:6379
JWT_SECRET=change-this-secret
FRONTEND_URL=http://localhost:5173
```

Run:

```bash
cd backend
go mod tidy
go run .
```

Backend runs on `http://localhost:8080`.

## 2. Frontend

Requirements:
- Node.js 18+

Run:

```bash
cd frontend
npm install
npm run dev
```

Frontend runs on `http://localhost:5173`.

## 3. Test realtime

1. Create an account.
2. Log in.
3. Create a poll.
4. Copy the generated poll link.
5. Open the link in two browser tabs.
6. Vote in one tab.
7. The result should update in the other tab without refreshing.

## Notes

MongoDB stores users, polls and votes. Redis stores live vote counts and publishes poll-update events. The Go WebSocket hub receives Redis events and broadcasts them to connected viewers.
