export const initialElements = () => [
    {
        id: 'src',
        sourcePosition: 'right',
        type: 'input',
        className: 'dark-node',
        data: { label: 'Source' },
        position: { x: 0, y: 80 },
    },
    {
        id: 'dst',
        targetPosition: 'left',
        type: 'output',
        data: { label: 'Destination' },
        position: { x: 250, y: 0 },
    },
    {
        id: 'internet',
        targetPosition: 'left',
        type: 'output',
        data: { label: 'Internet' },
        position: { x: 250, y: 160 },
    },

    {
        id: 'src-dst',
        source: 'src',
        target: 'dst',
        style: { stroke: 'green' },
        animated: true,
    },
    {
        id: 'src-internet',
        source: 'src',
        target: 'internet',

        style: { stroke: 'red' },
        animated: false,
    },
];
