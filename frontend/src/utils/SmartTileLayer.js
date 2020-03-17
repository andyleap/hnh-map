import L, {Bounds, LatLng, Point, Util, Browser} from "leaflet"

export const SmartTileLayer = L.TileLayer.extend({
    cache: {},
    invalidTile: "",
    map: 0,

    getTileUrl: function(coords) {
		return this.getTrueTileUrl(coords, this._getZoomForUrl());
    },

    getTrueTileUrl: function(coords, zoom) {
        var data = {
			r: Browser.retina ? '@2x' : '',
			s: this._getSubdomain(coords),
			x: coords.x,
            y: coords.y,
            map: this.map,
            z: zoom
        };
		if (this._map && !this._map.options.crs.infinite) {
			var invertedY = this._globalTileRange.max.y - coords.y;
			if (this.options.tms) {
				data['y'] = invertedY;
			}
			data['-y'] = invertedY;
        }
        
        data['cache'] = this.cache[data['map'] + ':'+ data['x'] + ':' + data['y'] + ':' + data['z']];

        if(!data['cache'] || data['cache'] == -1) {
            return this.invalidTile;
        }

		return Util.template(this._url, Util.extend(data, this.options));
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

        var tile = this._tiles[key];
        if(tile) {
            tile.el.src = this.getTrueTileUrl({x: x, y: y}, z);
        }
    }
});