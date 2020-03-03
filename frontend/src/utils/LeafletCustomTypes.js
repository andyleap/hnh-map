import L, {Bounds, LatLng, Point} from "leaflet"
import {getTileUrl} from "../main";

export const TileSize = 100;
export const HnHMaxZoom = 6;
export const HnHMinZoom = 1;

export const CustomGridLayer = L.GridLayer.extend({
    displayGridCoordinates: false,

    createTile: function (coords) {
        let element = document.createElement("div");
        element.width = TileSize;
        element.height = TileSize;
        element.classList.add("map-tile");

        let scaleFactor = Math.pow(2, HnHMaxZoom - coords.z);
        let topLeft = {x: coords.x * scaleFactor, y: coords.y * scaleFactor};
        let bottomRight = {x: topLeft.x + scaleFactor - 1, y: topLeft.y + scaleFactor - 1};

        // Async image loading
        let imageElement = document.createElement("img");
        imageElement.width = imageElement.height = TileSize - 1;  // Compensate border
        imageElement.style.display = "none";                      // Hide until loaded
        imageElement.id = `grid_${coords.x}_${coords.y}`;
        element.appendChild(imageElement);

        let asyncImage = new Image();
        asyncImage.onload = () => {
            imageElement.src = asyncImage.src;
            imageElement.style.display = "block";
            imageElement.classList.remove("load-error");
        };
        asyncImage.onerror = () => {
            imageElement.classList.add("load-error");
        };
        asyncImage.src = getTileUrl(topLeft.x, topLeft.y, coords.z);

        let text = `(${topLeft.x};${topLeft.y})`;
        if (scaleFactor !== 1) {
            text += `<br>(${bottomRight.x};${bottomRight.y})`;
        }

        let textElement = document.createElement("div");
        textElement.classList.add("map-tile-text");
        textElement.innerHTML = text;
        textElement.style.display = this.displayGridCoordinates ? "block" : "none";
        element.appendChild(textElement);
        return element;
    },

    showGridCoordinates(value) {
        this.displayGridCoordinates = value;

        // Update existing
        let display = value ? "block" : "none";
        document.querySelectorAll(".map-tile-text").forEach(it => it.style.display = display);
    },

    reloadTilesAround(gcList) {
        let gridsToReload = {};
        gcList.forEach(gc => {
            for (let i = -1; i <= 1; i++) {
                for (let j = -1; j <= 1; j++) {
                    let tmp = {x: gc.x + i, y: gc.y + j};
                    gridsToReload[`${tmp.x}_${tmp.y}`] = tmp;
                }
            }
        });
        Object.values(gridsToReload)
            .forEach(gc => {
                let imageElement = document.getElementById(`grid_${gc.x}_${gc.y}`);
                if (imageElement && imageElement.classList.contains("load-error")) {
                    let asyncImage = new Image();
                    asyncImage.onload = () => {
                        imageElement.src = asyncImage.src;
                        imageElement.style.display = "block";
                        imageElement.classList.remove("load-error");
                    };
                    asyncImage.src = getTileUrl(gc.x, gc.y, HnHMaxZoom);
                }
            });
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