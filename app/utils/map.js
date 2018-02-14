export default function(Model)
{
    return function(data)
    {
        // Merge the nodes into a list and create objects out of them
        return data.map(function(obj) {
            return Model.create(obj);
        });
    }
};
