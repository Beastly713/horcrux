#!/bin/bash
set -e # Exit on error

# 1. Setup
echo "---  Building Horcrux..."
go build -o horcrux main.go

# 2. Create Dummy Assets
echo "---  Creating Test Assets..."
echo "This is a super secret diary entry." > secret.txt

# Create a dummy PNG (200x200) using a tiny Go one-liner
# We need an image large enough to hold the horcrux shards
cat <<EOF > generate_png.go
package main
import (
    "image"
    "image/color"
    "image/png"
    "os"
)
func main() {
    img := image.NewNRGBA(image.Rect(0, 0, 500, 500))
    for i := 0; i < 500*500; i++ {
        img.Set(i%500, i/500, color.NRGBA{R: 100, G: 150, B: 200, A: 255})
    }
    f, _ := os.Create("carrier.png")
    png.Encode(f, img)
    f.Close()
}
EOF
go run generate_png.go
rm generate_png.go

# 3. Test Steganography Split
echo "---  Splitting file into Images..."
./horcrux split secret.txt -n 3 -t 2 --carrier-image carrier.png

if [ ! -f "secret_1_of_3.png" ]; then
    echo " Failed to create stego images"
    exit 1
fi

echo " Created secret_1_of_3.png (and others)"

# 4. Test Bind (Automatic Extraction)
echo "---  Binding from Images..."
# Delete original to ensure we are actually restoring it
rm secret.txt

# Bind using the images
./horcrux bind .

# 5. Verify
if [ -f "secret.txt" ]; then
    CONTENT=$(cat secret.txt)
    if [ "$CONTENT" == "This is a super secret diary entry." ]; then
        echo " SUCCESS: Secret restored correctly from images!"
    else
        echo " FAILURE: Content mismatch: $CONTENT"
        exit 1
    fi
else
    echo " FAILURE: secret.txt was not recreated."
    exit 1
fi

# 6. Clean up
rm horcrux secret.txt carrier.png secret_*_of_*.png
echo "---  Verification Complete"
