package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	bold        = "\033[1m"
)

var reader = bufio.NewReader(os.Stdin)

func main() {
	printBanner()

	fmt.Printf("\n%s%sWelcome to wikigo setup!%s\n\n", bold, colorCyan, colorReset)
	fmt.Println("This interactive script will help you set up wikigo for your needs.")
	fmt.Println()

	// Ask what they want to do
	fmt.Printf("%s%sWhat would you like to do?%s\n\n", bold, colorYellow, colorReset)
	fmt.Println("  1) Build all binaries")
	fmt.Println("  2) Quick start (build and run server)")
	fmt.Println("  3) Production deployment setup")
	fmt.Println("  4) Initialize database and index packages")
	fmt.Println("  5) Exit")
	fmt.Println()

	choice := prompt("Enter your choice [1-5]")

	switch choice {
	case "1":
		buildBinaries()
	case "2":
		quickStart()
	case "3":
		productionSetup()
	case "4":
		databaseSetup()
	case "5":
		fmt.Println("\n" + colorGreen + "Goodbye!" + colorReset)
		os.Exit(0)
	default:
		fmt.Printf("\n%sInvalid choice. Please run again.%s\n", colorRed, colorReset)
		os.Exit(1)
	}
}

func printBanner() {
	banner := `
 ██╗    ██╗██╗██╗  ██╗██╗ ██████╗  ██████╗
 ██║    ██║██║██║ ██╔╝██║██╔════╝ ██╔═══██╗
 ██║ █╗ ██║██║█████╔╝ ██║██║  ███╗██║   ██║
 ██║███╗██║██║██╔═██╗ ██║██║   ██║██║   ██║
 ╚███╔███╔╝██║██║  ██╗██║╚██████╔╝╚██████╔╝
  ╚══╝╚══╝ ╚═╝╚═╝  ╚═╝╚═╝ ╚═════╝  ╚═════╝ `

	fmt.Println(colorPurple + banner + colorReset)
	fmt.Println(colorCyan + "Multi-language Documentation Viewer" + colorReset)
}

func prompt(message string) string {
	fmt.Printf("%s%s:%s ", colorBlue, message, colorReset)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func promptYesNo(message string) bool {
	response := prompt(message + " [y/N]")
	return strings.ToLower(response) == "y" || strings.ToLower(response) == "yes"
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func buildBinaries() {
	fmt.Printf("\n%s%s=== Building Binaries ===%s\n\n", bold, colorGreen, colorReset)

	binaries := []struct {
		name string
		path string
		desc string
	}{
		{"serve", "./cmd/serve", "Documentation server"},
		{"crawl", "./cmd/crawl", "Go module crawler"},
		{"crawljs", "./cmd/crawljs", "JavaScript/TypeScript crawler"},
		{"crawlrs", "./cmd/crawlrs", "Rust crate crawler"},
		{"queryjs", "./cmd/queryjs", "Query JS/TS packages"},
		{"queryrs", "./cmd/queryrs", "Query Rust crates"},
	}

	for _, bin := range binaries {
		fmt.Printf("Building %s%s%s (%s)...\n", colorCyan, bin.name, colorReset, bin.desc)
		if err := runCommand("go", "build", "-o", bin.name, bin.path); err != nil {
			fmt.Printf("%sError building %s: %v%s\n", colorRed, bin.name, err, colorReset)
			os.Exit(1)
		}
	}

	fmt.Printf("\n%s✓ All binaries built successfully!%s\n", colorGreen, colorReset)
	fmt.Printf("\nBinaries are in the current directory:\n")
	for _, bin := range binaries {
		fmt.Printf("  ./%s\n", bin.name)
	}
}

func quickStart() {
	fmt.Printf("\n%s%s=== Quick Start ===%s\n\n", bold, colorGreen, colorReset)

	// Build serve binary
	fmt.Printf("Building %sserve%s binary...\n", colorCyan, colorReset)
	if err := runCommand("go", "build", "-o", "serve", "./cmd/serve"); err != nil {
		fmt.Printf("%sError building serve: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	// Ask for configuration
	addr := prompt("Server address (default: :8080)")
	if addr == "" {
		addr = ":8080"
	}

	dbPath := ""
	if promptYesNo("Use database for indexing?") {
		dbPath = prompt("Database path (default: wikigo.db)")
		if dbPath == "" {
			dbPath = "wikigo.db"
		}
	}

	dir := prompt("Directory to serve (default: current directory)")
	if dir == "" {
		dir = "."
	}

	// Build command
	args := []string{"-addr", addr}
	if dbPath != "" {
		args = append(args, "-db", dbPath)
	}
	args = append(args, "-dir", dir)

	fmt.Printf("\n%s✓ Starting wikigo server...%s\n\n", colorGreen, colorReset)
	fmt.Printf("Server will be available at: %shttp://localhost%s%s\n\n", colorCyan, addr, colorReset)
	fmt.Printf("Press Ctrl+C to stop the server.\n\n")

	cmd := exec.Command("./serve", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("%sError running server: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
}

func productionSetup() {
	fmt.Printf("\n%s%s=== Production Deployment Setup ===%s\n\n", bold, colorGreen, colorReset)

	// Build binaries
	fmt.Println("Step 1: Building production binaries with optimizations...")
	if err := runCommand("go", "build", "-ldflags=-s -w", "-o", "serve", "./cmd/serve"); err != nil {
		fmt.Printf("%sError building: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	fmt.Printf("%s✓ Built serve binary%s\n\n", colorGreen, colorReset)

	// Installation paths
	fmt.Println("Step 2: Installation paths")
	installPath := prompt("Install path for binary (default: /usr/local/bin/wikigo-serve)")
	if installPath == "" {
		installPath = "/usr/local/bin/wikigo-serve"
	}

	dataDir := prompt("Data directory (default: /var/lib/wikigo)")
	if dataDir == "" {
		dataDir = "/var/lib/wikigo"
	}

	// systemd setup
	fmt.Println("\nStep 3: systemd service")
	if promptYesNo("Set up systemd service?") {
		setupSystemd(installPath, dataDir)
	}

	// Caddy setup
	fmt.Println("\nStep 4: Reverse proxy")
	if promptYesNo("Set up Caddy reverse proxy?") {
		setupCaddy()
	}

	// Summary
	fmt.Printf("\n%s%s=== Setup Complete ===%s\n\n", bold, colorGreen, colorReset)
	fmt.Println("Next steps:")
	fmt.Printf("1. Copy binary: %ssudo cp serve %s%s\n", colorCyan, installPath, colorReset)
	fmt.Printf("2. Create data directory: %ssudo mkdir -p %s%s\n", colorCyan, dataDir, colorReset)
	fmt.Printf("3. Set permissions: %ssudo chown -R wikigo:wikigo %s%s\n", colorCyan, dataDir, colorReset)
	fmt.Printf("4. Start service: %ssudo systemctl start wikigo%s\n", colorCyan, colorReset)
}

func setupSystemd(installPath, dataDir string) {
	servicePath := "/etc/systemd/system/wikigo.service"

	user := prompt("Service user (default: wikigo)")
	if user == "" {
		user = "wikigo"
	}

	group := prompt("Service group (default: wikigo)")
	if group == "" {
		group = "wikigo"
	}

	addr := prompt("Server address (default: :8080)")
	if addr == "" {
		addr = ":8080"
	}

	fmt.Printf("\n%sSystemd service file: %s%s\n", colorYellow, servicePath, colorReset)
	fmt.Println("\nYou need to:")
	fmt.Printf("1. Create user: %ssudo useradd -r -s /bin/false %s%s\n", colorCyan, user, colorReset)
	fmt.Printf("2. Copy service: %ssudo cp deployment/wikigo.service %s%s\n", colorCyan, servicePath, colorReset)
	fmt.Printf("3. Edit service file with your values\n")
	fmt.Printf("4. Reload systemd: %ssudo systemctl daemon-reload%s\n", colorCyan, colorReset)
	fmt.Printf("5. Enable service: %ssudo systemctl enable wikigo%s\n", colorCyan, colorReset)
}

func setupCaddy() {
	domain := prompt("Your domain (e.g., docs.example.com)")

	fmt.Printf("\n%sCaddyfile configuration:%s\n\n", colorYellow, colorReset)
	fmt.Println("You need to:")
	fmt.Printf("1. Edit %sdeployment/Caddyfile%s\n", colorCyan, colorReset)
	fmt.Printf("2. Replace 'your-domain.com' with '%s%s%s'\n", colorCyan, domain, colorReset)
	fmt.Printf("3. Copy to Caddy: %ssudo cp deployment/Caddyfile /etc/caddy/Caddyfile%s\n", colorCyan, colorReset)
	fmt.Printf("4. Reload Caddy: %ssudo systemctl reload caddy%s\n", colorCyan, colorReset)
}

func databaseSetup() {
	fmt.Printf("\n%s%s=== Database Setup ===%s\n\n", bold, colorGreen, colorReset)

	dbPath := prompt("Database path (default: wikigo.db)")
	if dbPath == "" {
		dbPath = "wikigo.db"
	}

	// Check if database exists
	if _, err := os.Stat(dbPath); err == nil {
		if !promptYesNo(fmt.Sprintf("Database %s already exists. Continue anyway?", dbPath)) {
			return
		}
	}

	// Ask what to index
	fmt.Println("\nWhat would you like to index?")
	fmt.Println("  1) Go modules from proxy.golang.org")
	fmt.Println("  2) NPM packages")
	fmt.Println("  3) Rust crates")
	fmt.Println("  4) All of the above")

	choice := prompt("Enter your choice [1-4]")

	switch choice {
	case "1":
		indexGoModules(dbPath)
	case "2":
		indexNPMPackages(dbPath)
	case "3":
		indexRustCrates(dbPath)
	case "4":
		if promptYesNo("Index Go modules?") {
			indexGoModules(dbPath)
		}
		if promptYesNo("Index NPM packages?") {
			indexNPMPackages(dbPath)
		}
		if promptYesNo("Index Rust crates?") {
			indexRustCrates(dbPath)
		}
	default:
		fmt.Printf("%sInvalid choice%s\n", colorRed, colorReset)
		return
	}

	fmt.Printf("\n%s✓ Database setup complete!%s\n", colorGreen, colorReset)
	fmt.Printf("\nYou can now run the server with:\n")
	fmt.Printf("  %s./serve -db %s%s\n", colorCyan, dbPath, colorReset)
}

func indexGoModules(dbPath string) {
	fmt.Println("\nBuilding Go module crawler...")
	if err := runCommand("go", "build", "-o", "crawl", "./cmd/crawl"); err != nil {
		fmt.Printf("%sError building crawler: %v%s\n", colorRed, err, colorReset)
		return
	}

	maxStr := prompt("Maximum modules to index (0 for unlimited, recommended: 100 for testing)")
	if maxStr == "" {
		maxStr = "100"
	}

	fmt.Printf("\n%sIndexing Go modules...%s\n", colorYellow, colorReset)
	args := []string{"-db", dbPath}
	if maxStr != "0" {
		args = append(args, "-max", maxStr)
	}

	if err := runCommand("./crawl", args...); err != nil {
		fmt.Printf("%sError indexing: %v%s\n", colorRed, err, colorReset)
	}
}

func indexNPMPackages(dbPath string) {
	fmt.Println("\nBuilding NPM crawler...")
	if err := runCommand("go", "build", "-o", "crawljs", "./cmd/crawljs"); err != nil {
		fmt.Printf("%sError building crawler: %v%s\n", colorRed, err, colorReset)
		return
	}

	packages := prompt("Enter NPM package names (comma-separated, e.g., express,react,vue)")
	pkgList := strings.Split(packages, ",")

	for _, pkg := range pkgList {
		pkg = strings.TrimSpace(pkg)
		if pkg == "" {
			continue
		}

		fmt.Printf("\n%sIndexing %s...%s\n", colorYellow, pkg, colorReset)
		if err := runCommand("./crawljs", "-npm", pkg, "-db", dbPath); err != nil {
			fmt.Printf("%sError indexing %s: %v%s\n", colorRed, pkg, err, colorReset)
		}
	}
}

func indexRustCrates(dbPath string) {
	fmt.Println("\nBuilding Rust crate crawler...")
	if err := runCommand("go", "build", "-o", "crawlrs", "./cmd/crawlrs"); err != nil {
		fmt.Printf("%sError building crawler: %v%s\n", colorRed, err, colorReset)
		return
	}

	crates := prompt("Enter crate names (comma-separated, e.g., serde,tokio,actix-web)")
	crateList := strings.Split(crates, ",")

	for _, crate := range crateList {
		crate = strings.TrimSpace(crate)
		if crate == "" {
			continue
		}

		fmt.Printf("\n%sIndexing %s...%s\n", colorYellow, crate, colorReset)
		if err := runCommand("./crawlrs", "-crate", crate, "-db", dbPath); err != nil {
			fmt.Printf("%sError indexing %s: %v%s\n", colorRed, crate, err, colorReset)
		}
	}
}

func init() {
	// Check if we're in the right directory
	if _, err := os.Stat("go.mod"); err != nil {
		fmt.Printf("%sError: Not in wikigo project directory%s\n", colorRed, colorReset)
		fmt.Println("Please run this from the wikigo root directory")
		os.Exit(1)
	}

	// Check if Go is installed
	if _, err := exec.LookPath("go"); err != nil {
		fmt.Printf("%sError: Go is not installed%s\n", colorRed, colorReset)
		fmt.Println("Please install Go 1.25.0 or later")
		os.Exit(1)
	}
}
