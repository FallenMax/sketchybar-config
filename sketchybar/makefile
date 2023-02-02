
config:
	@echo "apply config"
	brew services restart sketchybar 
	./update_sketchybar.mjs

watch:
  # https://stackoverflow.com/questions/3004811/how-do-you-run-multiple-programs-in-parallel-from-a-bash-script
	@echo "start watching"
	(trap 'kill 0' SIGINT; chokidar 'sketchybarrc' -c 'brew services restart sketchybar' & chokidar 'update_sketchybar.mjs' -c './update_sketchybar.mjs' )
