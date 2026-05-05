# RabbitMQ Broadcasting System

Sistem manajemen produk dan transaksi real-time berbasis microservices yang menggunakan RabbitMQ sebagai message broker dan menerapkan pola **Transactional Outbox**.

## Fitur Utama
- **Real-time Updates**: Menggunakan WebSockets untuk memperbarui tampilan frontend secara instan saat ada produk baru.
- **Transactional Outbox Pattern**: Menjamin konsistensi data antara database (MySQL) dan antrian pesan (RabbitMQ).
- **Authentication**: Integrasi sistem autentikasi (simulasi LDAP/JWT).
- **Modern UI**: Dibangun dengan Vue 3, Tailwind CSS 4, dan Shadcn Vue untuk tampilan yang premium.

## Arsitektur
1. **API Service**: Menangani request CRUD produk dan menyimpan event ke tabel `outbox`.
2. **Worker Service**: Membaca data dari tabel `outbox` secara berkala dan mengirimkannya ke RabbitMQ.
3. **Consumer Service**: Mendengarkan pesan dari RabbitMQ dan meneruskannya ke WebSocket service.
4. **WS Service**: Menjaga koneksi WebSocket dengan klien frontend untuk siaran real-time.
5. **Auth Service**: Menangani proses login dan validasi token.

## Prasyarat
- [Node.js](https://nodejs.org/) (v20+)
- [Go](https://golang.org/) (v1.21+)
- [Docker & Docker Compose](https://www.docker.com/)

## Cara Instalasi & Menjalankan

### 1. Jalankan Infrastruktur (Docker)
Gunakan Docker Compose untuk menjalankan MySQL, RabbitMQ, dan Nginx:
```bash
docker-compose up -d
```

### 2. Konfigurasi Environment
Salin `.env.example` ke `.env` (jika ada) atau pastikan nilai di `.env` sudah benar:
```env
DB_PASSWORD=rootpassword
DB_NAME=transaction_db
RABBITMQ_USER=guest
RABBITMQ_PASS=guest
```

### 3. Jalankan Backend (Go)
Anda perlu menjalankan setiap servis backend. Disarankan menggunakan terminal terpisah:
```bash
# Jalankan di folder masing-masing servis
go run backend/api/main.go
go run backend/auth/main.go
go run backend/ws/main.go
go run backend/worker/main.go
go run backend/consumer/main.go
```

### 4. Jalankan Frontend (Vue)
```bash
npm install
npm run dev
```
Aplikasi akan berjalan di `http://localhost:5173` (atau port lain yang muncul di terminal).

## Pengembangan
- **Linting**: `npm run lint`
- **Type-check**: `npm run type-check`
- **Build**: `npm run build`

## Teknologi
- **Frontend**: Vue 3, Vite, Tailwind CSS 4, Radix UI, Lucide Icons.
- **Backend**: Go (Gin, GORM, RabbitMQ-Go).
- **Database**: MySQL.
- **Proxy**: Nginx.
