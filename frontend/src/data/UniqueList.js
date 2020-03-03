/**
 * Elements should have unique field "id"
 */
export class UniqueList {
    constructor() {
        this.elements = {};
    }

    update(dataList, addCallback, removeCallback, updateCallback) {
        let elementsToAdd = dataList
            .filter(it => this.elements[it.id] === undefined);
        let elementsToRemove = Object.keys(this.elements)
            .filter(it => dataList.find(up => ("" + up.id) === it) === undefined)
            .map(id => this.elements[id]);
        if (removeCallback) {
            elementsToRemove.forEach(it => removeCallback(it))
        }
        if (updateCallback) {
            dataList
                .forEach(newElement => {
                    let oldElement = this.elements[newElement.id];
                    if (oldElement) {
                        updateCallback(oldElement, newElement)
                    }
                });
        }
        if (addCallback) {
            elementsToAdd.forEach(it => addCallback(it));
        }
        elementsToRemove.forEach(it => delete this.elements[it.id]);
        elementsToAdd.forEach(it => this.elements[it.id] = it);
    }

    getElements() {
        return Object.values(this.elements);
    }

    byId(id) {
        return this.elements[id];
    }
}