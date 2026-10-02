#!/bin/sh
# Prepare the principal's photo for the About page.
#
#   scripts/portrait.sh path/to/photo.jpg       # crop a little above the middle
#   FOCUS=0.1 scripts/portrait.sh photo.jpg     # 0 keeps the top of the photo, 1 the bottom
#
# Writes static/images/opt/portrait-{480,960}.{webp,jpg}: a 4:5 crop, turned
# upright, with the camera's metadata (EXIF: GPS position, device, time)
# removed. Once all four files exist the About page shows the photo instead of
# its placeholder: reload in -dev, rebuild to deploy. The original is not
# copied anywhere; only static/images/opt is embedded in the binary.
# Needs ffmpeg and ffprobe on the workstation.
set -eu

src=${1:-}
focus=${FOCUS:-0.3}
if [ -z "$src" ] || [ ! -f "$src" ]; then
	echo "usage: [FOCUS=0..1] $0 path/to/photo" >&2
	exit 2
fi
case $focus in
0 | 1 | 0.[0-9] | 0.[0-9][0-9]) ;;
*)
	echo "FOCUS must be between 0 and 1, e.g. 0.3" >&2
	exit 2
	;;
esac

root=$(cd "$(dirname "$0")/.." && pwd)
out="$root/static/images/opt"

# Phones store portrait shots sideways plus an EXIF orientation tag. Older
# ffmpeg ignores the tag, so apply it here and switch automatic rotation off.
orientation=$(ffprobe -v error -select_streams v:0 -read_intervals "%+#1" \
	-show_entries frame_tags=Orientation -of default=nw=1:nk=1 "$src" 2>/dev/null | head -n 1 | tr -d '[:space:]')
case $orientation in
2) turn="hflip," ;;
3) turn="hflip,vflip," ;;
4) turn="vflip," ;;
5) turn="transpose=0," ;;
6) turn="transpose=1," ;;
7) turn="transpose=3," ;;
8) turn="transpose=2," ;;
*) turn="" ;;
esac

crop="crop=w='min(iw,ih*4/5)':h='min(ih,iw*5/4)':x='(iw-ow)/2':y='(ih-oh)*$focus'"

for w in 480 960; do
	h=$((w * 5 / 4))
	vf="${turn}${crop},scale=$w:$h:flags=lanczos"
	ffmpeg -v error -y -noautorotate -i "$src" -frames:v 1 -vf "$vf" -map_metadata -1 \
		-c:v libwebp -quality 80 "$out/portrait-$w.webp"
	ffmpeg -v error -y -noautorotate -i "$src" -frames:v 1 -vf "$vf" -map_metadata -1 \
		-pix_fmt yuvj420p -q:v 3 "$out/portrait-$w.jpg"
done

ls -l "$out"/portrait-*
echo "done: the About page now shows the photo (go run . -dev to check the crop)"
