var SmartTileLayer = L.TileLayer.extend({
    cache: {},
    invalidTile: "",
    layer: 0,

    getTileUrl: function(coords) {
        console.log(coords);
		return this.getTrueTileUrl(coords, this._getZoomForUrl());
    },

    getTrueTileUrl: function(coords, zoom) {
        var data = {
			r: L.Browser.retina ? '@2x' : '',
			s: this._getSubdomain(coords),
			x: coords.x,
            y: coords.y,
            layer: this.layer,
            z: zoom
        };
		if (this._map && !this._map.options.crs.infinite) {
			var invertedY = this._globalTileRange.max.y - coords.y;
			if (this.options.tms) {
				data['y'] = invertedY;
			}
			data['-y'] = invertedY;
        }
        
        data['cache'] = this.cache[data['layer'] + ':'+ data['x'] + ':' + data['y'] + ':' + data['z']];

        if(!data['cache'] || data['cache'] == -1) {
            return this.invalidTile;
        }

		return L.Util.template(this._url, L.Util.extend(data, this.options));
    },

    refresh: function(x, y, z)  {
        var zoom = z,
		maxZoom = this.options.maxZoom,
		zoomReverse = this.options.zoomReverse,
		zoomOffset = this.options.zoomOffset;

		if (zoomReverse) {
			zoom = maxZoom - zoom;
        }

        zoom = zoom + zoomOffset;
        
        var key = x + ':' + y + ':' + zoom;

        if(this._tiles) {
            var tile = this._tiles[key];
            if(tile) {
                tile.el.src = this.getTrueTileUrl({x: x, y: y}, z);
            }
        }
    }
});