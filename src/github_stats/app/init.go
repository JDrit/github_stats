package app

import (
    "github.com/revel/revel"
    "strconv"
    "bytes"
    "time"
)

func init() {
	// Filters is the default set of global filters.
	revel.Filters = []revel.Filter{
		revel.PanicFilter,             // Recover from panics and display an error page instead.
		revel.RouterFilter,            // Use the routing table to select the right Action
		revel.FilterConfiguringFilter, // A hook for adding or removing per-Action filters.
		revel.ParamsFilter,            // Parse parameters into Controller.Params.
		revel.SessionFilter,           // Restore and write the session cookie.
		revel.FlashFilter,             // Restore and write the flash cookie.
		revel.ValidationFilter,        // Restore kept validation errors and save new ones from cookie.
		revel.I18nFilter,              // Resolve the requested language
		HeaderFilter,                  // Add some security based headers
		revel.InterceptorFilter,       // Run interceptors around the action.
		revel.CompressFilter,          // Compress the result.
		revel.ActionInvoker,           // Invoke the action.
	}
    
    revel.TemplateFuncs["delim"] = func(num int ) string {
        var buffer bytes.Buffer
        runes := []rune(strconv.Itoa(num))
        for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
            runes[i], runes[j] = runes[j], runes[i]
        }
        revStr := string(runes)
        for i := 0 ; i < len(revStr) ; i++ {
            if i % 3 == 0 && i != 0 {
                buffer.WriteString(",")
            }
            buffer.WriteString(string(revStr[i]))
        }
        runes1 := []rune(buffer.String())
        for i, j := 0, len(runes1) - 1 ; i < j ; i, j = i+1, j-1 {
            runes1[i], runes1[j] = runes1[j], runes1[i]
        }
        return string(runes1)

    }

    revel.TemplateFuncs["neq"] = func(a, b interface{}) bool {
        return a != b
    }

    revel.TemplateFuncs["formatDate"] = func(timestamp int64) string {
        return time.Unix(timestamp, 0).String()
    }

	// register startup functions with OnAppStart
	// ( order dependent )
	// revel.OnAppStart(InitDB())
	// revel.OnAppStart(FillCache())
}

// TODO turn this into revel.HeaderFilter
// should probably also have a filter for CSRF
// not sure if it can go in the same filter or not
var HeaderFilter = func(c *revel.Controller, fc []revel.Filter) {
	// Add some common security headers
	c.Response.Out.Header().Add("X-Frame-Options", "SAMEORIGIN")
	c.Response.Out.Header().Add("X-XSS-Protection", "1; mode=block")
	c.Response.Out.Header().Add("X-Content-Type-Options", "nosniff")

	fc[0](c, fc[1:]) // Execute the next filter stage.
}
