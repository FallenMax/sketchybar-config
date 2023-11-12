
build:
	# use esbuild to build src/*.ts -> update_sketchybar.mjs
	@echo "build"
	esbuild --bundle --target=esnext --format=esm --outfile=update_sketchybar.mjs  --platform=node --external:zx src/update_sketchybar.ts 

config:
	@echo "apply config"
	brew services restart sketchybar 
	./update_sketchybar.mjs

watch:
  # https://stackoverflow.com/questions/3004811/how-do-you-run-multiple-programs-in-parallel-from-a-bash-script
	@echo "start watching"
	( \
		trap 'kill 0' SIGINT; \
		chokidar 'src/*.ts' -c 'make build' & \
		chokidar 'sketchybarrc' -c 'brew services restart sketchybar' & \
		chokidar '*.mjs' -c './update_sketchybar.mjs' \
	)
