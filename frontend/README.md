### Requirements

1. NodeJS 12+

### Configuration

You should set `MAP_ENDPOINT` in the file `src/config.js` to the domain you will use

For example
```js
export const MAP_ENDPOINT = 'http://ec2-1-2-3-4.eu-central-1.compute.amazonaws.com';
```

### Building

1. `npm install`
2. `npm run build`

Frontend will be built into `dist/` folder

### Compiles and hot-reloads for development

Run `npm run serve`

