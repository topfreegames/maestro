package:
	mkdir -p pkg
	mkdir -p charts/maestro/charts
	cd charts && cp -r maestro-* maestro/charts && helm package * && mv *.tgz ../pkg/ && rm -rf maestro/charts
	cd ..

clean:
	@rm -r pkg
