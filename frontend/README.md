# StreamMaxing Frontend

React + TypeScript frontend for managing Twitch notification settings.

## Setup
1. Copy `.env.example` to `.env` and set `VITE_API_URL`
2. Install dependencies: `npm install`
3. Run dev server: `npm run dev`
4. Build for production: `npm run build`

## Deployment
```bash
npm run build
aws s3 sync dist/ s3://streammaxing-frontend --delete
aws cloudfront create-invalidation --distribution-id XXX --paths "/*"
```

## Project Structure
```
frontend/
├── src/
│   ├── components/      # React components
│   ├── services/        # API client
│   ├── types/           # TypeScript types
│   ├── App.tsx          # Main app component
│   └── main.tsx         # Entry point
└── public/              # Static assets
```
