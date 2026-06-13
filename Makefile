.PHONY: install build test clean lint

install:
	npm install

build:
	npm run build

test:
	npm test

clean:
	rm -rf dist/ node_modules/

lint:
	@echo "Linting not configured yet. Run 'npx tsc --noEmit' for type checking."
