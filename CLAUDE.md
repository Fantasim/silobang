For the back-end (golang):
1. Prioritize the maintanability of the code in your plan over the working result. Better take more time doing something because it's clean and sustainable than something quick.

2. Don't write any hardcoded value use the constant package for that.

3. In this project immutability, security (against data corruption), and movability (be able to move a topic folder in other project and it would adapt really quickly)

4. Whatever new feeature you add, you have to e2e end test it extremely well. and unit test it also extrmely well. SECURITY IS A MUST FOR US. WE ARE A SECURITY COMPANY.

5. Add proper logging about everything going on, there is never enough logs. To helps now what's going for every step. Make sure to use a consistent logging type with all informations we need. If you come accross a log that is not consistant with the rest of the codebase, improve it on your way. (Except in testing of course)


On the Front end (js)

1. Don't write any hardcoded value use the constant package for that.

2. Prioritize the maintanability of the code in your plan over the working result. Better take more time doing something because it's clean and sustainable than something quick.

3. We have a design system, use it. Dont re-invent shit as its a waste of token and time. (in src/components for the UI components and in src/styles/tokens.css)

4. No matter what you do in the front-end you have to be the best UI/UX designer ever. You don't compromise you offer the best.