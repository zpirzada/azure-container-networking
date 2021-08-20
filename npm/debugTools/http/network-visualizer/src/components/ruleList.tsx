import * as React from 'react';
import {
    DetailsList,
    DetailsListLayoutMode,
    IDetailsListStyles,
    IDetailsHeaderProps,
    ConstrainMode,
    IColumn
} from '@fluentui/react/lib/DetailsList';
import { IRenderFunction } from '@fluentui/react/lib/Utilities';
import { TooltipHost } from '@fluentui/react/lib/Tooltip';
import { mergeStyleSets } from '@fluentui/react/lib/Styling';
import { IDetailsColumnRenderTooltipProps } from '@fluentui/react/lib/DetailsList';

export interface Rule {
    key: number | string;
    chain: string;
    protocol: string;
    dPort: string;
    sPort: string;
    allowed: string;
    direction: string;
}

const gridStyles: Partial<IDetailsListStyles> = {
    root: {
        overflowX: 'scroll',
        selectors: {
            '& [role=grid]': {
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'start',
                height: '60vh',
            },
        },
    },
    headerWrapper: {
        flex: '0 0 auto',
    },
    contentWrapper: {
        flex: '1 1 auto',
        overflowY: 'auto',
        overflowX: 'hidden',
    },
};

const classNames = mergeStyleSets({
    header: {
        margin: 0,
    },
    row: {
        flex: '0 0 auto',
    },
});


const LOREM_IPSUM = (
    'lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor incididunt ut ' +
    'labore et dolore magna aliqua ut enim ad minim veniam quis nostrud exercitation ullamco laboris nisi ut ' +
    'aliquip ex ea commodo consequat duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore ' +
    'eu fugiat nulla pariatur excepteur sint occaecat cupidatat non proident sunt in culpa qui officia deserunt '
).split(' ');
let loremIndex = 0;
const lorem = (wordCount: number): string => {
    const startIndex = loremIndex + wordCount > LOREM_IPSUM.length ? 0 : loremIndex;
    loremIndex = startIndex + wordCount;
    return LOREM_IPSUM.slice(startIndex, loremIndex).join(' ');
};
const allItems = Array.from({ length: 10 }).map((item, index) => ({
    key: index,
    chain: lorem(4),
    protocol: lorem(4),
    dPort: lorem(4),
    sPort: lorem(4),
    allowed: lorem(4),
    direction: lorem(4),
}));

const columns: IColumn[] = [
    {
        key: 'column1',
        name: 'Chain',
        fieldName: 'chain',
        isRowHeader: true,
        isResizable: true,
        isPadded: true,
        isSorted: true,
        minWidth: 200,
    },
    {
        key: 'column2',
        name: 'Protocol',
        fieldName: 'protocol',
        minWidth: 100,
        isResizable: true,
        isPadded: true,
    },
    {
        key: 'column3',
        name: 'Source port',
        fieldName: 'sPort',
        minWidth: 100,
        isResizable: true,
        isPadded: true,
    },
    {
        key: 'column4',
        name: 'Destination port',
        fieldName: 'dPort',
        minWidth: 100,
        isResizable: true,
        isPadded: true,
    },
    {
        key: 'column5',
        name: 'Allowed',
        fieldName: 'allowed',
        minWidth: 100,
        isResizable: true,
        isPadded: true,
    },
    {
        key: 'column6',
        name: 'Direction',
        fieldName: 'direction',
        minWidth: 100,
        isResizable: true,
        isPadded: true,
    },
];
const onItemInvoked = (item: Rule): void => {
    alert('Item invoked: ' + item.chain);
};
const onRenderDetailsHeader: IRenderFunction<IDetailsHeaderProps> = (props, defaultRender) => {
    if (!props) {
        return null;
    }
    const onRenderColumnHeaderTooltip: IRenderFunction<IDetailsColumnRenderTooltipProps> = tooltipHostProps => (
        <TooltipHost {...tooltipHostProps} />
    );
    return defaultRender!({
        ...props,
        onRenderColumnHeaderTooltip,
    });
};

export const RuleList: React.FunctionComponent = () => {
    return (
        <div>
            <h2 className={classNames.header}>Hit Rules</h2>
            <DetailsList
                items={allItems}
                columns={columns}
                setKey="set"
                layoutMode={DetailsListLayoutMode.fixedColumns}
                constrainMode={ConstrainMode.unconstrained}
                onRenderDetailsHeader={onRenderDetailsHeader}
                selectionPreservedOnEmptyClick
                styles={gridStyles}
                ariaLabelForSelectionColumn="Toggle selection"
                ariaLabelForSelectAllCheckbox="Toggle selection for all items"
                onItemInvoked={onItemInvoked}
            />
        </div>
    );
};