package psref_test

import (
	"context"
	"fmt"

	"github.com/dennwc/psref"
)

func ExampleClient() {
	c := psref.NewClient()

	ctx := context.Background()

	// https://psref.lenovo.com/Detail/ThinkPad/ThinkPad_X1_Carbon_Gen_10?M=21CB000AUS
	const code = "21CB000AUS"

	m, err := c.ModelByCode(ctx, code)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Product %d (%s):\n\tCPU: %s\n\tRAM: %s\n\tGPU: %s\n\tDisk: %s\n",
		m.ID, m.Key,
		m.DetailByName("Processor"), m.DetailByName("Memory"),
		m.DetailByName("Graphics"), m.DetailByName("Storage"),
	)
	// Output:
	// Product 1972 (ThinkPad_X1_Carbon_Gen_10):
	// 	CPU: Intel Core i5-1240P, 12C (4P + 8E) / 16T, P-core 1.7 / 4.4GHz, E-core 1.2 / 3.3GHz, 12MB
	//	RAM: 16GB Soldered LPDDR5-5200
	//	GPU: Integrated Intel Iris Xe Graphics
	//	Disk: 256GB SSD M.2 2280 PCIe 4.0x4 NVMe Opal2
}
