package helmx

import "github.com/variantdev/chartify"

func (r *Runner) Chartify(release, dirOrChart string, opts ...chartify.ChartifyOption) (string, error) {
	rr := chartify.New(chartify.HelmBin(r.helmBin), chartify.UseHelm3(r.isHelm3))

	return rr.Chartify(release, dirOrChart, opts...)
}
