<div align="center">
  <h1>Uzeltok 📦🚀</h1>
  <p><strong>A blazingly fast, single-binary, zero-dependency file uploader for self-hosters.</strong></p>

  <!-- Badges -->
  <p>
    <a href="https://github.com/momoirodouhu/Uzeltok/releases"><img src="https://img.shields.io/github/v/release/momoirodouhu/Uzeltok?style=flat-square" alt="Latest Release"></a>
    <a href="https://golang.org/"><img src="https://img.shields.io/github/go-mod/go-version/momoirodouhu/Uzeltok?style=flat-square" alt="Go Version"></a>
    <a href="https://github.com/momoirodouhu/Uzeltok/blob/main/LICENSE"><img src="https://img.shields.io/github/license/momoirodouhu/Uzeltok?style=flat-square" alt="License"></a>
    <a href="https://github.com/momoirodouhu/Uzeltok/actions/workflows/release.yml"><img src="https://img.shields.io/github/actions/workflow/status/momoirodouhu/Uzeltok/release.yml?style=flat-square" alt="Build Status"></a>
  </p>

  <p>
    <a href="#-why-uzeltok">Features</a> •
    <a href="#-getting-started">Getting Started</a> •
    <a href="#-usage">Usage</a>
  </p>
</div>

---

**Uzeltok** is a lightweight, high-performance file sharing platform designed exclusively for self-hosting. It requires absolutely no database, no complex configurations, and no user sign-ups. Just run the binary, and you are ready to share files securely.

Perfect for homelabs, quick file transfers between devices, or sharing assets with clients without relying on third-party cloud services.

## ✨ Features

- ⚡️ **Single Binary**: The entire application (backend, frontend, templates) is compiled into a single executable file.
- 🗄️ **No Database Required**: All metadata and files are stored directly on your file system.
- 🚫 **No Sign-up**: Frictionless experience. Share links immediately.
- ⏳ **Expiring Links**: Generate secure links that auto-expire after a set duration (e.g., 1 hour, 7 days) to prevent stale data.
- 📥 **Drop Links**: Create special "Drop" links that allow anyone (like your clients or friends) to securely upload files *to* your server without needing an account.
- 🐳 **Docker Ready**: Pre-built, multi-architecture Docker images (`ghcr.io`) based on secure `distroless` containers.

## 🚀 Getting Started

### Method 1: Docker Compose (Recommended)

The easiest way to get started is using Docker.

1. Create a `docker-compose.yml` file:
```yaml
version: '3.8'

services:
  uzeltok:
    image: ghcr.io/momoirodouhu/uzeltok:latest
    container_name: uzeltok
    ports:
      - "8080:8080"
    environment:
      - ADMIN_PASSWORD=change_here
    volumes:
      - ./data:/app/data
    restart: unless-stopped
```

2. Run the container:
```bash
docker-compose up -d
```

*(Alternatively, run it via a single command: `docker run -d -p 8080:8080 -e ADMIN_PASSWORD=change_here -v ./data:/app/data ghcr.io/momoirodouhu/uzeltok:latest`)*

### Method 2: Standalone Binary

1. Download the latest binary for your OS from the [Releases](https://github.com/momoirodouhu/Uzeltok/releases) page.
2. Make it executable (Linux/macOS): `chmod +x Uzeltok-server`
3. Run the server:
```bash
ADMIN_PASSWORD=change_here ./Uzeltok-server
```

## ⚙️ Configuration

Uzeltok keeps it simple by relying exclusively on Environment Variables.

| Variable | Default Value | Description |
| :--- | :--- | :--- |
| `ADMIN_PASSWORD` | *(empty)* | **Required.** The password used to access the `/admin` dashboard. If left empty, all admin dashboard access is completely blocked (HTTP 401). |
| `UPLOAD_MAX_BYTES` | `33554432` (32 MiB) | Maximum allowed upload size in bytes for both admin uploads and public drop uploads. Requests that exceed this limit are rejected. |
| `UZELTOK_PORT` | `8080` | The port the HTTP server binds to. |
| `UZELTOK_DATA_DIR` | `./data` | The directory where files and metadata will be persistently stored. |

## 📖 Usage

### Accessing the Admin Dashboard
1. Navigate to `http://localhost:8080/admin`
2. You will be prompted for basic authentication. The username can be anything; the password is the value you set for `ADMIN_PASSWORD`.

### Share Links vs. Drop Links

Uzeltok handles two main workflows:

- 📤 **Share Link**: Use this when *you* want to send files. Upload your file(s) via the admin dashboard, and share the generated URL. The recipient gets a clean page to download them.
- 📥 **Drop Link**: Use this when you want *others* to send you files. Create an empty Drop Link and share the URL. The recipient gets a clean upload page where they can drop files securely into your server.

## 🤝 Contributing

Contributions are what make the open source community such an amazing place to learn, inspire, and create. Any contributions you make are **greatly appreciated**.

### Local Development Setup

1. **Clone the repository:**
   ```bash
   git clone https://github.com/momoirodouhu/Uzeltok.git
   cd Uzeltok
   ```
2. **Run the development server:**
   ```bash
   ADMIN_PASSWORD=dev go run cmd/server/main.go
   ```
3. **Run tests:**
   *(Currently, we are looking for contributors to help write unit tests!)*
   ```bash
   go test ./...
   ```

### Contribution Guidelines
1. Fork the Project.
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`).
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`).
4. Push to the Branch (`git push origin feature/AmazingFeature`).
5. Open a Pull Request!

*Please ensure your code passes standard Go formatting (`go fmt`) and linting (`go vet`).*

## 📄 License

Distributed under the MIT License. See `LICENSE` for more information.
