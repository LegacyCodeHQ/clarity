package com.example.model;

public class Cart {
    private PaymentMethod paymentMethod;
    private Discount discount;
    // DeliveryOption should be ignored in comments.
    private String note = "DISCOUNT should be ignored in string";

    public double total() {
        return 0.0;
    }
}
