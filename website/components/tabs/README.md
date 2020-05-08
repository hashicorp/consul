# Tabs Component

> An MDX-compatible Tabs component

This React component renders tabbed content. [Example](https://p176.p0.n0.cdn.getcloudapp.com/items/E0ubRrlq/Screen%20Recording%202020-05-08%20at%2004.40%20PM.gif?v=a1f576d2c207f4312ca14adbce8a53ac)

## Usage

- Use the `<Tabs>` tag in your markdown file to begin a tabbed content section.
- Use the `<Tab>` tag with a `heading` prop to separate your markdown

### Important

A line must be skipped between the `<Tab>` and your markdown (for both above and below said markdown). [This is a limitation of MDX also pointed out by the Docusaurus folks 🔗 ](https://v2.docusaurus.io/docs/markdown-features/#multi-language-support-code-blocks). There is work currently happening with the mdx parser to eliminate this issue.

### Example

```jsx
<Tabs>
  <Tab heading="CLI command">
    {/* Intentionally skipped line.. */}
    ### Content
    {/* Intentionally skipped line.. */}
  </Tab>
  <Tab heading="API call using cURL">### Content</Tab>
</Tabs>
```

### Component Props

`<Tabs>` can be provided any arbitrary `children` so long as the `heading` prop is present the React or HTML tag used to wrap markdown, that said, we provide the `<Tab>` component to separate your tab content without rendering extra, unnecessary markup.

This works:

```jsx
<Tabs>
  <Tab heading="CLI command">### Content</Tab>
  ....
</Tabs>
```

This _does not_ work, as the `<Tab>` element is missing a `heading` prop:

```jsx
<Tabs>
  <Tab>### Content</Tab>
  ....
</Tabs>
```
