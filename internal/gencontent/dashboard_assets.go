package gencontent

import _ "embed"

//go:embed dashboard_assets/style.css
var dashboardStyle []byte

//go:embed dashboard_assets/app.js
var dashboardScript []byte
