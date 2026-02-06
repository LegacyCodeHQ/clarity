package com.example.app;

import com.example.model.*;
import com.example.util.Helper;
import java.util.List;

public class App {
    private final Helper helper = new Helper();
    private final Cart cart = new Cart();
    private final List<String> items = List.of();

    public String summary() {
        return helper.format(cart.total(), items.size());
    }
}
