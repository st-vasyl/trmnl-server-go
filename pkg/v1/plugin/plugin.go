package plugin

// Plugin is the contract every screen plugin must satisfy.
// Each plugin owns its own data-fetching and rendering logic;
// the worker calls Render for each screen the plugin declares.
type Plugin interface {
	// Name returns the plugin's config identifier (e.g. "weather", "twelvedata").
	Name() string

	// Screens returns every screen name this plugin can render.
	// Multi-symbol plugins return one entry per symbol (e.g. "twelvedata_googl").
	Screens() []string

	// Render fetches data and writes a PNG to outputPath.
	// screen is one of the values returned by Screens().
	Render(screen, outputPath string, voltage float32) error
}
