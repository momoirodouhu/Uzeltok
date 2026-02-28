package buildinfo

var (
	// Version is populated by ldflags during build.
	Version = "dev"
	// GithubURL is populated by ldflags during build.
	GithubURL = "https://github.com/momoirodouhu/Uzeltok"
)

// Info holds the build information.
type Info struct {
	Version   string
	GithubURL string
}

// Get returns the current build information.
func Get() Info {
	info := Info{
		Version:   Version,
		GithubURL: GithubURL,
	}
	
	return info
}
