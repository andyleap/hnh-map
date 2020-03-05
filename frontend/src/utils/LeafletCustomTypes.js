import L, {Bounds, LatLng, Point} from "leaflet"
import {getTileUrl} from "../main";

export const TileSize = 100;
export const HnHMaxZoom = 6;
export const HnHMinZoom = 1;

export const GridCoordLayer = L.GridLayer.extend({
    createTile: function (coords) {
        let element = document.createElement("div");
        element.width = TileSize;
        element.height = TileSize;
        element.classList.add("map-tile");

        let scaleFactor = Math.pow(2, HnHMaxZoom - coords.z);
        let topLeft = {x: coords.x * scaleFactor, y: coords.y * scaleFactor};
        let bottomRight = {x: topLeft.x + scaleFactor - 1, y: topLeft.y + scaleFactor - 1};

        let text = `(${topLeft.x};${topLeft.y})`;
        if (scaleFactor !== 1) {
            text += `<br>(${bottomRight.x};${bottomRight.y})`;
        }

        let textElement = document.createElement("div");
        textElement.classList.add("map-tile-text");
        textElement.innerHTML = text;
        textElement.style.display = 'block';
        element.appendChild(textElement);
        return element;
    }
});

export const ImageIcon = L.Icon.extend({
    options: {
        iconSize: [32, 32],
        iconAnchor: [16, 16],
    }
});

const latNormalization = 90.0 * TileSize / 2500000.0;
const lngNormalization = 180.0 * TileSize / 2500000.0;

const HnHProjection = {
    project: function (latlng) {
        return new Point(latlng.lat / latNormalization, latlng.lng / lngNormalization);
    },

    unproject: function (point) {
        return new LatLng(point.x * latNormalization, point.y * lngNormalization);
    },

    bounds: (function () {
        return new Bounds([-latNormalization, -lngNormalization], [latNormalization, lngNormalization]);
    })()
};

export const HnHCRS = L.extend({}, L.CRS.Simple, {
    projection: HnHProjection
});